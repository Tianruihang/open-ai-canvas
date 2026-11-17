// Package providercatalog 解析本地静态目录 JSON，按推广期过滤，并把推荐模型
// 的人民币成本换算为积分成本与加价售价。
//
// 目录决定管理员 API Key 会被发往哪个 Base URL，因此这里只接受 https 地址，
// 并对每个字段做长度与取值校验；协议是否被当前版本支持由调用方判断。
package providercatalog

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultLogoBase 只用于拼接目录里的相对 logo 路径，不参与目录加载。
	DefaultLogoBase = "https://ai.yingce.tv"
	// DefaultCatalogFile 是数据目录下静态目录的默认相对位置。
	DefaultCatalogFile = "provider-catalog/catalog.json"

	envCatalogFile     = "CANVAS_PROVIDER_CATALOG_FILE"
	envCatalogLogoBase = "CANVAS_PROVIDER_CATALOG_LOGO_BASE"
	envCatalogDisabled = "CANVAS_PROVIDER_CATALOG_DISABLED"
)

type SourceState string

const (
	SourceLive        SourceState = "live"
	SourceCache       SourceState = "cache"
	SourceUnavailable SourceState = "unavailable"
	SourceDisabled    SourceState = "disabled"
)

// Snapshot 是最近一次成功解析的目录。
type Snapshot struct {
	Catalog   *Catalog
	FetchedAt time.Time
	State     SourceState
	LastError string
}

// Source 从本地静态 JSON 文件加载目录。每次读取都会重新解析文件，
// 因此编辑文件后无需重启，刷新目录即可看到变更。
type Source struct {
	path     string
	logoBase string
	disabled bool
	now      func() time.Time

	mu       sync.Mutex
	snapshot Snapshot
}

// NewSource 按以下优先级确定静态目录路径：
//  1. CANVAS_PROVIDER_CATALOG_FILE 显式指定；
//  2. 数据目录下的 provider-catalog/catalog.json（生产环境）；
//  3. 仓库内 scripts/provider-catalog/catalog.json（本地开发回退）。
func NewSource(dataDir string) *Source {
	logoBase := strings.TrimSpace(os.Getenv(envCatalogLogoBase))
	if logoBase == "" {
		logoBase = DefaultLogoBase
	}
	return &Source{
		path:     resolveCatalogPath(dataDir),
		logoBase: logoBase,
		disabled: strings.TrimSpace(os.Getenv(envCatalogDisabled)) == "1",
		now:      time.Now,
	}
}

func resolveCatalogPath(dataDir string) string {
	if explicit := strings.TrimSpace(os.Getenv(envCatalogFile)); explicit != "" {
		return explicit
	}
	if dir := strings.TrimSpace(dataDir); dir != "" {
		candidate := filepath.Join(dir, filepath.FromSlash(DefaultCatalogFile))
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		// 本地开发回退：从 backend/ 或仓库根运行时使用仓库内的静态目录。
		for _, fallback := range []string{
			filepath.Join("scripts", "provider-catalog", "catalog.json"),
			filepath.Join("..", "scripts", "provider-catalog", "catalog.json"),
		} {
			if _, err := os.Stat(fallback); err == nil {
				return fallback
			}
		}
		return candidate
	}
	return filepath.FromSlash(DefaultCatalogFile)
}

// NewSourceForTest 允许测试指定本地文件路径、logo 前缀与时间。
func NewSourceForTest(path, logoBase string, now func() time.Time) *Source {
	return &Source{path: path, logoBase: logoBase, now: now}
}

// Now 返回服务端当前时间，供调用方按推广期过滤目录。
func (s *Source) Now() time.Time { return s.now() }

// Get 重新读取本地文件；读取或解析失败时保留上一次成功快照。
func (s *Source) Get(_ context.Context) Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.disabled {
		return Snapshot{State: SourceDisabled}
	}
	s.loadLocked()
	return s.snapshot
}

// Refresh 与 Get 语义相同：本地文件没有 ETag/缓存，重新读取即为刷新。
func (s *Source) Refresh(ctx context.Context) Snapshot {
	return s.Get(ctx)
}

func (s *Source) loadLocked() {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		s.markFailureLocked(err)
		return
	}
	catalog, warnings, err := Parse(raw, s.logoBase)
	for _, warning := range warnings {
		log.Printf("provider catalog: %s", warning)
	}
	if err != nil {
		s.markFailureLocked(err)
		return
	}
	s.snapshot = Snapshot{Catalog: catalog, FetchedAt: s.now(), State: SourceLive}
}

func (s *Source) markFailureLocked(err error) {
	log.Printf("provider catalog load failed: %v", err)
	s.snapshot.LastError = err.Error()
	if s.snapshot.Catalog != nil {
		s.snapshot.State = SourceCache
	} else {
		s.snapshot.State = SourceUnavailable
	}
}
