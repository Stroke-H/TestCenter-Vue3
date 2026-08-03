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

function Resolve-Command {
  param([string]$Name, [string[]]$FallbackPaths = @())
  $command = Get-Command $Name -ErrorAction SilentlyContinue
  if ($command) {
    return $command.Source
  }

  foreach ($fallbackPath in $FallbackPaths) {
    if (Test-Path -LiteralPath $fallbackPath -PathType Leaf) {
      return $fallbackPath
    }
  }

  throw "缺少命令: $Name"
}

function Wait-TcpPort {
  param(
    [string]$HostName,
    [int]$Port,
    [int]$TimeoutSeconds,
    [System.Diagnostics.Process]$Process
  )

  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  while ((Get-Date) -lt $deadline) {
    if ($Process -and $Process.HasExited) {
      throw "Go 后端启动失败，进程已退出。"
    }

    $client = New-Object System.Net.Sockets.TcpClient
    try {
      $asyncResult = $client.BeginConnect($HostName, $Port, $null, $null)
      if ($asyncResult.AsyncWaitHandle.WaitOne(500)) {
        $client.EndConnect($asyncResult)
        return
      }
    } catch {
      Start-Sleep -Milliseconds 300
    } finally {
      $client.Close()
    }
  }

  throw "Go 后端启动超时，端口 $Port 未就绪。"
}

$ProjectRoot = (Resolve-Path -LiteralPath $ProjectRoot).Path
Set-Location $ProjectRoot

Assert-Command "node"
Assert-Command "npm"

$npmCommand = Resolve-Command "npm.cmd" @("C:\Program Files\nodejs\npm.cmd")
$goCommand = Resolve-Command "go" @("C:\Program Files\Go\bin\go.exe")

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

Write-Host "等待 Go 后端端口就绪..."
Wait-TcpPort -HostName "127.0.0.1" -Port 8080 -TimeoutSeconds 30 -Process $backend

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
