param(
  [string]$Output = "dist/plugins",
  [string]$Targets = "current",
  [string]$Plugin = "",
  [switch]$Combined,
  [string]$SignKey = "",
  [string]$KeyID = "",
  [string]$GenerateKey = ""
)

if (-not [string]::IsNullOrWhiteSpace($GenerateKey)) {
  go run ./plugins/cmd/pluginpack --generate-key $GenerateKey
  if ($LASTEXITCODE -ne 0) { throw "plugin key generation failed with exit code $LASTEXITCODE" }
  return
}
if (-not [string]::IsNullOrWhiteSpace($Plugin)) {
  if ([string]::IsNullOrWhiteSpace($SignKey) -ne [string]::IsNullOrWhiteSpace($KeyID)) { throw "-SignKey and -KeyID must be set together" }
  $toolArgs = @("--plugin", $Plugin, "--output", $Output, "--targets", $Targets)
  if ($Combined) { $toolArgs += "--combined" }
  if (-not [string]::IsNullOrWhiteSpace($SignKey)) { $toolArgs += @("--sign-key", $SignKey, "--key-id", $KeyID) }
  go run ./plugins/cmd/pluginpack @toolArgs
  if ($LASTEXITCODE -ne 0) { throw "plugin packaging failed with exit code $LASTEXITCODE" }
  return
}

New-Item -ItemType Directory -Force -Path $Output | Out-Null
$signKey = $env:MEERKIT_PLUGIN_SIGN_KEY
$keyId = $env:MEERKIT_PLUGIN_KEY_ID
if ([string]::IsNullOrWhiteSpace($signKey) -ne [string]::IsNullOrWhiteSpace($keyId)) {
  throw "MEERKIT_PLUGIN_SIGN_KEY and MEERKIT_PLUGIN_KEY_ID must be set together"
}
$plugins = Get-ChildItem -Path "plugins" -Directory | Where-Object {
  $_.Name -ne "template" -and (Test-Path (Join-Path $_.FullName "meerkit-plugin.yaml"))
}
if ($plugins.Count -eq 0) { throw "No publishable plugins found under plugins/" }
foreach ($plugin in $plugins) {
  if ([string]::IsNullOrWhiteSpace($signKey)) {
    go run ./plugins/cmd/pluginpack --plugin $plugin.FullName --output $Output --targets $Targets
  } else {
    go run ./plugins/cmd/pluginpack --plugin $plugin.FullName --output $Output --targets $Targets --sign-key $signKey --key-id $keyId
  }
  if ($LASTEXITCODE -ne 0) { throw "plugin packaging failed with exit code $LASTEXITCODE" }
}
