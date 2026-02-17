@echo off
setlocal

set "SCRIPT_DIR=%~dp0"
set "REPO_ROOT=%SCRIPT_DIR%..\.."

pushd "%REPO_ROOT%" >nul 2>&1
if errorlevel 1 (
    echo [ERROR] failed to cd repo root: "%REPO_ROOT%"
    exit /b 1
)

where go >nul 2>&1
if errorlevel 1 (
    echo [ERROR] go command not found in PATH.
    popd >nul 2>&1
    exit /b 1
)

go run internal\integration_test\main.go %*
set "EXIT_CODE=%ERRORLEVEL%"

popd >nul 2>&1
exit /b %EXIT_CODE%
