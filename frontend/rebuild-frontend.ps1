Param(
  [switch]$SkipInstall,
  [switch]$InstallOnly
)

# Windows PowerShell rebuild script for the frontend
# Usage examples:
#   ./rebuild-frontend.ps1              # install + build
#   ./rebuild-frontend.ps1 -SkipInstall # reuse existing node_modules
#   ./rebuild-frontend.ps1 -InstallOnly # only install deps

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

Push-Location $PSScriptRoot

function Detect-PackageManager {
  if (Get-Command pnpm -ErrorAction SilentlyContinue) { return 'pnpm' }
  elseif (Get-Command yarn -ErrorAction SilentlyContinue) { return 'yarn' }
  else { return 'npm' }
}

$pm = Detect-PackageManager
Write-Host "[frontend] package manager: $pm"

if (-not $SkipInstall) {
  Write-Host "[frontend] installing dependencies..."
  switch ($pm) {
    'yarn' { yarn install --frozen-lockfile 2>$null; if ($LASTEXITCODE -ne 0) { yarn install } }
    'pnpm' { pnpm install --frozen-lockfile 2>$null; if ($LASTEXITCODE -ne 0) { pnpm install } }
    default { npm install }
  }
}

if ($InstallOnly) {
  Write-Host "[frontend] install-only mode complete"
  Pop-Location
  exit 0
}

Write-Host "[frontend] cleaning previous build artifacts..."
Remove-Item -Recurse -Force dist -ErrorAction SilentlyContinue | Out-Null
Remove-Item -Force tsconfig.tsbuildinfo -ErrorAction SilentlyContinue | Out-Null

Write-Host "[frontend] building..."
Switch ($pm) {
  'yarn' { yarn build }
  'pnpm' { pnpm build }
  default { npm run build }
}

Write-Host "[frontend] build complete. Dist sample files:"
Get-ChildItem dist -Recurse -File | Select-Object -First 30 | ForEach-Object { Write-Host $_.FullName }

Pop-Location
