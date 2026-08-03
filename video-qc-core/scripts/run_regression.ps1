param(
  [string]$FFmpeg = "ffmpeg",
  [string]$FFprobe = "ffprobe",
  [string]$Go = "go"
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
$Tmp = Join-Path $Root "tmp\regression"
New-Item -ItemType Directory -Force -Path $Tmp | Out-Null

function Invoke-FFmpeg {
  param(
    [string[]]$Arguments
  )
  $previous = $ErrorActionPreference
  $ErrorActionPreference = "Continue"
  try {
    & $FFmpeg @Arguments
    $exitCode = $LASTEXITCODE
  }
  finally {
    $ErrorActionPreference = $previous
  }
  if ($exitCode -ne 0) {
    throw "ffmpeg failed with exit code $exitCode"
  }
}

function Invoke-QC {
  param(
    [string]$Name,
    [string]$Profile = "default"
  )
  $inputPath = Join-Path $Tmp "$Name.mp4"
  $reportPath = Join-Path $Tmp "$Name-report.json"
  & $Go run .\cmd\video-qc check --input $inputPath --profile ".\configs\$Profile.json" --work-dir (Join-Path $Tmp "$Name-work") --ffmpeg $FFmpeg --ffprobe $FFprobe --output $reportPath
  if ($LASTEXITCODE -ne 0) {
    throw "video-qc failed for $Name"
  }
  $report = Get-Content -LiteralPath $reportPath -Raw | ConvertFrom-Json
  [PSCustomObject]@{
    Name = $Name
    Profile = $Profile
    Status = $report.status
    Events = ($report.events | ForEach-Object { $_.type }) -join ","
    Score = $report.summary.score
  }
}

Push-Location $Root
try {
  Invoke-FFmpeg @("-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "color=c=black:size=320x180:rate=25:d=1.2", "-f", "lavfi", "-i", "testsrc=size=320x180:rate=25:d=1", "-filter_complex", "[0:v][1:v]concat=n=2:v=1:a=0", "-pix_fmt", "yuv420p", (Join-Path $Tmp "black.mp4"))
  Invoke-FFmpeg @("-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "testsrc=size=320x180:rate=25:d=1", "-f", "lavfi", "-i", "color=c=white:size=320x180:rate=25:d=1.2", "-f", "lavfi", "-i", "color=c=blue:size=320x180:rate=25:d=1.2", "-filter_complex", "[0:v][1:v][2:v]concat=n=3:v=1:a=0", "-pix_fmt", "yuv420p", (Join-Path $Tmp "white.mp4"))
  Invoke-FFmpeg @("-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "testsrc=size=320x180:rate=25:d=1", "-f", "lavfi", "-i", "color=c=green:size=320x180:rate=25:d=1.2", "-f", "lavfi", "-i", "testsrc2=size=320x180:rate=25:d=1", "-filter_complex", "[0:v][1:v][2:v]concat=n=3:v=1:a=0", "-pix_fmt", "yuv420p", (Join-Path $Tmp "green.mp4"))
  Invoke-FFmpeg @("-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "testsrc=size=320x180:rate=25:d=1", "-f", "lavfi", "-i", "color=c=blue:size=320x180:rate=25:d=2.2", "-f", "lavfi", "-i", "testsrc2=size=320x180:rate=25:d=1", "-filter_complex", "[0:v][1:v][2:v]concat=n=3:v=1:a=0", "-pix_fmt", "yuv420p", (Join-Path $Tmp "freeze.mp4"))
  Invoke-FFmpeg @("-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "testsrc=size=320x180:rate=25:d=3", "-itsoffset", "0.7", "-f", "lavfi", "-i", "sine=frequency=1000:duration=2.3", "-map", "0:v", "-map", "1:a", "-shortest", "-pix_fmt", "yuv420p", (Join-Path $Tmp "avsync.mp4"))

  $results = @()
  $results += Invoke-QC -Name "black"
  $results += Invoke-QC -Name "white"
  $results += Invoke-QC -Name "green"
  $results += Invoke-QC -Name "freeze"
  $results += Invoke-QC -Name "avsync" -Profile "deep"
  $results | Format-Table -AutoSize
}
finally {
  Pop-Location
}
