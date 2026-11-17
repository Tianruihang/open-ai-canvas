@echo off
setlocal EnableExtensions

rem ------------------------------------------------------------
rem  YingCe local dev launcher for Windows.
rem  Opens two windows:
rem    backend  : go run ./cmd/server   -> 127.0.0.1:8080
rem    frontend : bun run dev           -> 0.0.0.0:3000
rem  The real startup logic stays in scripts\start-local.ps1
rem  (data dir, Go caches, proxy import, port checks, dep install).
rem  Local development only. Do not use on a public deployment.
rem ------------------------------------------------------------

cd /d "%~dp0"
set "ROOT=%~dp0"
set "STARTER=%ROOT%scripts\start-local.ps1"

if not exist "%STARTER%" (
    echo [ERROR] scripts\start-local.ps1 was not found.
    echo         Put start.bat in the repository root and run it again.
    goto :fail
)

where.exe go >nul 2>nul
if errorlevel 1 (
    echo [ERROR] "go" is not in PATH. Install Go 1.25+ first.
    goto :fail
)

where.exe bun >nul 2>nul
if errorlevel 1 (
    echo [ERROR] "bun" is not in PATH. Install Bun first.
    echo         Required version: see web/package.json packageManager field.
    goto :fail
)

echo.
echo ============================================================
echo  YingCe - local frontend + backend
echo ------------------------------------------------------------
echo  Backend  : http://127.0.0.1:8080
echo  Frontend : http://localhost:3000
echo  Register : http://localhost:3000/register
echo             first account automatically becomes admin
echo  Features : http://localhost:3000/admin/settings/features
echo  Stop     : press Ctrl+C in BOTH opened windows
echo ============================================================
echo.
echo Starting two PowerShell windows. First run may need a few minutes.
echo.

powershell.exe -NoProfile -NoLogo -ExecutionPolicy Bypass -File "%STARTER%"
if errorlevel 1 (
    echo.
    echo [WARN] Starter stopped with an error.
    echo        Common cause: port 3000 or 8080 is already in use,
    echo        or scripts\start-local.ps1 could not install web deps.
    echo        If services are already running, just open:
    echo        http://localhost:3000
    echo.
    pause
    exit /b 1
)

echo.
echo Waiting 8 seconds for Vite, then opening the browser...
timeout /t 8 /nobreak >nul
start "" "http://localhost:3000/register"
echo.
echo If the page shows an API error, wait for the backend window to
echo listen on 8080 and refresh the page.
echo This start.bat window can be closed; the two dev windows keep running.
echo.
pause
exit /b 0

:fail
echo.
echo Start aborted.
pause
exit /b 1
