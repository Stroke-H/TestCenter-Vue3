param(
  [string]$ProjectRoot = (Get-Location).Path
)

$ErrorActionPreference = "Stop"

function Import-DotEnv {
  param([string]$Path)
  if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
    Write-Host "未找到 .env.lan.local，将使用默认运行配置。"
    return
  }

  Get-Content -LiteralPath $Path | ForEach-Object {
    $line = $_.Trim()
    if ($line -eq "" -or $line.StartsWith("#")) {
      return
    }
    $index = $line.IndexOf("=")
    if ($index -le 0) {
      return
    }
    $name = $line.Substring(0, $index).Trim()
    $value = $line.Substring($index + 1).Trim().Trim('"').Trim("'")
    [Environment]::SetEnvironmentVariable($name, $value, "Process")
  }
}

function Assert-Command {
  param([string]$Name)
  if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
    throw "缺少命令: $Name"
  }
}

$ProjectRoot = (Resolve-Path -LiteralPath $ProjectRoot).Path
Set-Location $ProjectRoot

Assert-Command "node"
Assert-Command "npm"
Assert-Command "go"

$npmCommand = (Get-Command "npm").Source
$goCommand = (Get-Command "go").Source

Import-DotEnv -Path (Join-Path $ProjectRoot ".env.lan.local")

$env:TESTCENTER_BACKEND_HOST = "0.0.0.0"
$env:TESTCENTER_BACKEND_PORT = "8080"
$env:TESTCENTER_DB_CONFIG_FILE = Join-Path $ProjectRoot "server\data\database_config.json"

if (-not (Test-Path -LiteralPath "node_modules" -PathType Container)) {
  Write-Host "安装前端依赖..."
  & $npmCommand install
}

Write-Host "启动 Go 后端: http://localhost:8080"
$backend = Start-Process -FilePath $goCommand -ArgumentList @("run", "main.go") -WorkingDirectory (Join-Path $ProjectRoot "server") -NoNewWindow -PassThru

Start-Sleep -Seconds 2
if ($backend.HasExited) {
  throw "Go 后端启动失败，进程已退出。"
}

Write-Host "启动 Vite 前端: http://localhost:5173/dashboard"
$frontend = Start-Process -FilePath $npmCommand -ArgumentList @("run", "dev") -WorkingDirectory $ProjectRoot -NoNewWindow -PassThru

Write-Host ""
Write-Host "TestCenter 正在运行:"
Write-Host "  Frontend: http://localhost:5173/dashboard"
Write-Host "  Backend:  http://localhost:8080"
Write-Host ""
Write-Host "按 Ctrl+C 停止；如窗口关闭后进程仍在，可使用 taskkill 结束 node/go 进程。"

try {
  while ($true) {
    if ($backend.HasExited) {
      throw "Go 后端进程已退出。"
    }
    if ($frontend.HasExited) {
      throw "Vite 前端进程已退出。"
    }
    Start-Sleep -Seconds 2
  }
} finally {
  if ($backend -and -not $backend.HasExited) {
    Stop-Process -Id $backend.Id -Force
  }
  if ($frontend -and -not $frontend.HasExited) {
    Stop-Process -Id $frontend.Id -Force
  }
}
