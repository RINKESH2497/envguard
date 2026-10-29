@echo off
echo ========================================================
echo  envguard - GitHub Launch Initializer
echo ========================================================
echo.

where git >nul 2>nul
if %errorlevel% neq 0 (
    echo [!] Git was not found in your PATH.
    echo     To install Git run:
    echo     winget install --id Git.Git -e --source winget
    echo.
    echo     Then reopen this terminal and run init-git.bat again.
    pause
    exit /b 1
)

cd /d "%~dp0"

if not exist ".git" (
    echo [*] Initializing Git repository...
    git init -b main
) else (
    echo [*] Git repository already initialized.
)

echo [*] Staging all project files...
git add .

echo [*] Committing initial codebase...
git commit -m "feat: initial release of envguard - universal environment variable validator"

echo.
echo ========================================================
echo  SUCCESS! Repository is ready for GitHub.
echo ========================================================
echo.
echo Next steps:
echo 1. Create a new empty repository at https://github.com/new (named 'envguard')
echo 2. Run:
echo    git remote add origin https://github.com/YOUR_USERNAME/envguard.git
echo    git push -u origin main
echo.
pause
