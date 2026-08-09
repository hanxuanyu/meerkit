param(
  [string]$Output = "dist/releases",
  [string]$Targets = "",
  [string]$Version = ""
)

if ([string]::IsNullOrWhiteSpace($Targets)) { $Targets = "$(go env GOOS)/$(go env GOARCH)" }
if ([string]::IsNullOrWhiteSpace($Version)) {
  $Version = if ($env:MEERKIT_VERSION) { $env:MEERKIT_VERSION } else { "dev" }
}
New-Item -ItemType Directory -Force -Path $Output | Out-Null
npm --prefix web run build
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

foreach ($target in $Targets.Split(",")) {
  $parts = $target.Trim().Split("/")
  if ($parts.Count -ne 2) { throw "Invalid target: $target" }
  $goos, $goarch = $parts
  $stage = Join-Path ([System.IO.Path]::GetTempPath()) ("meerkit-release-" + [guid]::NewGuid())
  $releaseName = "meerkit-$Version-$goos-$goarch"
  $releaseDir = Join-Path $stage $releaseName
  New-Item -ItemType Directory -Force -Path (Join-Path $releaseDir "plugins") | Out-Null
  $binary = if ($goos -eq "windows") { "meerkit.exe" } else { "meerkit" }
  $plugincheckBinary = if ($goos -eq "windows") { "meerkit-plugincheck.exe" } else { "meerkit-plugincheck" }
  $oldGOOS, $oldGOARCH, $oldCGO = $env:GOOS, $env:GOARCH, $env:CGO_ENABLED
  $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = $goos, $goarch, "0"
  go build -trimpath -ldflags "-s -w -X main.version=$Version" -o (Join-Path $releaseDir $binary) .
  if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
  go build -trimpath -ldflags "-s -w" -o (Join-Path $releaseDir $plugincheckBinary) ./cmd/plugincheck
  $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = $oldGOOS, $oldGOARCH, $oldCGO
  if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
  & ./scripts/package-plugins.ps1 -Output (Join-Path $releaseDir "plugins") -Targets "$goos/$goarch"
  if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
  Copy-Item README.md, README.en.md, config.example.yaml -Destination $releaseDir
  Compress-Archive -Path $releaseDir -DestinationPath (Join-Path $Output "$releaseName.zip") -Force
  Remove-Item -Recurse -Force $stage
}
