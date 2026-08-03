param(
  [Parameter(Mandatory = $true)]
  [string]$Bundle,

  [string]$ProjectRoot = (Get-Location).Path
)

$ErrorActionPreference = "Stop"

function Assert-FileExists {
  param([string]$Path, [string]$Message)
  if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
    throw $Message
  }
}

function Assert-DirectoryExists {
  param([string]$Path, [string]$Message)
  if (-not (Test-Path -LiteralPath $Path -PathType Container)) {
    throw $Message
  }
}

$ProjectRoot = (Resolve-Path -LiteralPath $ProjectRoot).Path
if (-not (Test-Path -LiteralPath $Bundle -PathType Leaf)) {
  throw "迁移包不存在: $Bundle"
}
$BundlePath = (Resolve-Path -LiteralPath $Bundle).Path

Assert-FileExists -Path (Join-Path $ProjectRoot "package.json") -Message "当前目录不像 TestCenter 项目根目录，缺少 package.json: $ProjectRoot"
Assert-DirectoryExists -Path (Join-Path $ProjectRoot "server") -Message "当前目录不像 TestCenter 项目根目录，缺少 server 目录: $ProjectRoot"

Write-Host "TestCenter Windows 私有迁移包还原"
Write-Host "项目根目录: $ProjectRoot"
Write-Host "迁移包: $BundlePath"

tar -xzf $BundlePath -C $ProjectRoot
if ($LASTEXITCODE -ne 0) {
  throw "迁移包解压失败，tar exit code: $LASTEXITCODE"
}

$checks = @(
  ".env.lan.local",
  "drama_info.json",
  "server\data\database_config.json",
  "server\data\feishu_config.json",
  "server\data\ai_config.json",
  "backup\legacy",
  "report"
)

Write-Host ""
Write-Host "路径校验:"
foreach ($relativePath in $checks) {
  $fullPath = Join-Path $ProjectRoot $relativePath
  if (Test-Path -LiteralPath $fullPath) {
    Write-Host "  OK  $relativePath"
  } else {
    Write-Host "  MISS $relativePath"
  }
}

Write-Host ""
Write-Host "还原完成。下一步可以执行:"
Write-Host "  powershell -ExecutionPolicy Bypass -File .\scripts\start_windows.ps1"
