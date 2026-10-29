<#
.SYNOPSIS
    Initializes git repository and prepares envguard for GitHub launch.
#>

$ErrorActionPreference = "Stop"

Write-Host "🚀 Preparing envguard for GitHub launch..." -ForegroundColor Cyan

# Check if git is available
$gitCmd = Get-Command "git" -ErrorAction SilentlyContinue

if (-not $gitCmd) {
    Write-Host "⚠️  Git was not found in your PATH." -ForegroundColor Yellow
    Write-Host "   You can install Git quickly with:" -ForegroundColor White
    Write-Host "   winget install --id Git.Git -e --source winget" -ForegroundColor Green
    Write-Host "`n   After installing, re-run this script to initialize and push your repo." -ForegroundColor White
    exit 1
}

# Navigate to script directory (envguard root)
Set-Location $PSScriptRoot

# Initialize git if not already initialized
if (-not (Test-Path ".git")) {
    Write-Host "📁 Initializing new Git repository..." -ForegroundColor White
    git init -b main
} else {
    Write-Host "📁 Existing Git repository detected." -ForegroundColor White
}

# Add all files
Write-Host "📦 Staging files..." -ForegroundColor White
git add .

# Check if there are changes to commit
$status = git status --porcelain
if ($status) {
    Write-Host "💾 Creating initial commit..." -ForegroundColor White
    git commit -m "feat: initial release of envguard - universal environment variable validator"
    Write-Host "✅ Committed initial codebase!" -ForegroundColor Green
} else {
    Write-Host "ℹ️  Working tree is already clean." -ForegroundColor White
}

Write-Host "`n🎉 Ready to launch on GitHub!" -ForegroundColor Cyan
Write-Host "To link and push to your GitHub account:" -ForegroundColor White
Write-Host "1. Create a new empty repository named 'envguard' on https://github.com/new" -ForegroundColor White
Write-Host "2. Run the following commands:" -ForegroundColor White
Write-Host "   git remote add origin https://github.com/<YOUR-USERNAME>/envguard.git" -ForegroundColor Green
Write-Host "   git push -u origin main" -ForegroundColor Green
Write-Host "3. (Optional) Create a release tag:" -ForegroundColor White
Write-Host "   git tag v0.1.0" -ForegroundColor Green
Write-Host "   git push origin v0.1.0" -ForegroundColor Green
