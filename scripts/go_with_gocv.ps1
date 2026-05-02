param(
  [Parameter(ValueFromRemainingArguments = $true)]
  [string[]]$GoArgs
)

$ErrorActionPreference = "Stop"

function Get-EnvValue {
  param([string]$Name)
  $value = [Environment]::GetEnvironmentVariable($Name, "Process")
  if ([string]::IsNullOrWhiteSpace($value)) {
    $value = [Environment]::GetEnvironmentVariable($Name, "User")
  }
  if ([string]::IsNullOrWhiteSpace($value)) {
    $value = [Environment]::GetEnvironmentVariable($Name, "Machine")
  }
  return $value
}

function Get-OpenCVRoot {
  $root = Get-EnvValue "OPENCV_DIR"
  if (-not [string]::IsNullOrWhiteSpace($root)) {
    return $root
  }
  return $null
}

function Get-OpenCVInclude {
  $include = Get-EnvValue "OPENCV_INCLUDE"
  if (-not [string]::IsNullOrWhiteSpace($include)) {
    return $include
  }
  $root = Get-OpenCVRoot
  if ($root) {
    return (Join-Path $root "include")
  }
  return $null
}

function Get-OpenCVLib {
  $lib = Get-EnvValue "OPENCV_LIB"
  if (-not [string]::IsNullOrWhiteSpace($lib)) {
    return $lib
  }
  $root = Get-OpenCVRoot
  if ($root) {
    return (Join-Path $root "lib")
  }
  return $null
}

function Get-OpenCVBin {
  $bin = Get-EnvValue "OPENCV_BIN"
  if (-not [string]::IsNullOrWhiteSpace($bin)) {
    return $bin
  }
  $root = Get-OpenCVRoot
  if ($root) {
    return (Join-Path $root "bin")
  }
  return $null
}

function Get-OpenCVLibName {
  $name = Get-EnvValue "OPENCV_LIB_NAME"
  if (-not [string]::IsNullOrWhiteSpace($name)) {
    return $name
  }

  $libDir = Get-OpenCVLib
  if (-not $libDir -or -not (Test-Path $libDir)) {
    return $null
  }

  $candidate = Get-ChildItem -Path $libDir -Filter "opencv_world*.lib" |
    Sort-Object Name -Descending |
    Select-Object -First 1
  if ($candidate) {
    return [System.IO.Path]::GetFileNameWithoutExtension($candidate.Name)
  }
  return $null
}

if (-not $GoArgs -or $GoArgs.Count -eq 0) {
  throw "请传入 go 命令参数，例如：.\scripts\go_with_gocv.ps1 build -tags gocv ./cmd/..."
}

$includeDir = Get-OpenCVInclude
$libDir = Get-OpenCVLib
$binDir = Get-OpenCVBin
$libName = Get-OpenCVLibName

if ([string]::IsNullOrWhiteSpace($includeDir) -or [string]::IsNullOrWhiteSpace($libDir) -or [string]::IsNullOrWhiteSpace($libName)) {
  throw "缺少 OpenCV 环境变量。请至少设置 OPENCV_DIR，或分别设置 OPENCV_INCLUDE / OPENCV_LIB / OPENCV_LIB_NAME。"
}

$env:CGO_ENABLED = "1"
$env:CGO_CXXFLAGS = "-I`"$includeDir`""
$env:CGO_CPPFLAGS = $env:CGO_CXXFLAGS
$env:CGO_LDFLAGS = "-L`"$libDir`" -l$libName"

if (-not [string]::IsNullOrWhiteSpace($binDir)) {
  if ([string]::IsNullOrWhiteSpace($env:PATH)) {
    $env:PATH = $binDir
  } elseif (-not ($env:PATH.Split(';') | Where-Object { $_ -eq $binDir })) {
    $env:PATH = "$binDir;$env:PATH"
  }
}

Write-Host "使用 OpenCV include: $includeDir"
Write-Host "使用 OpenCV lib: $libDir"
Write-Host "使用 OpenCV lib name: $libName"
if (-not [string]::IsNullOrWhiteSpace($binDir)) {
  Write-Host "使用 OpenCV bin: $binDir"
}
Write-Host "执行命令: go $($GoArgs -join ' ')"

& go @GoArgs
exit $LASTEXITCODE
