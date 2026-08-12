$ErrorActionPreference = "Stop"
$u = "test_curl_" + (Get-Random -Max 99999)
$body = @{ username = $u; password = "test123456" } | ConvertTo-Json
# 写 UTF-8 无 BOM
$enc = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText("$env:TEMP\reg.json", $body, $enc)
$reg = curl.exe -s -m 15 -X POST "http://127.0.0.1:8080/api/auth/register" -H "Content-Type: application/json" --data-binary "@$env:TEMP\reg.json"
Write-Host "REGISTER RESP: $reg"
$obj = $reg | ConvertFrom-Json
if (-not $obj.token) { exit 1 }
$h = @{ Authorization = "Bearer $($obj.token)" }
$me = curl.exe -s -m 15 "http://127.0.0.1:8080/api/auth/me" -H "Authorization: Bearer $($obj.token)"
Write-Host "ME RESP: $me"
$g = curl.exe -s -m 15 "http://127.0.0.1:8080/api/growth/my-profile" -H "Authorization: Bearer $($obj.token)"
Write-Host "GROWTH MY-PROFILE RESP: $g"
