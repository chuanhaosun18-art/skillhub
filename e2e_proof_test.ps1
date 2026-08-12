# End-to-end test: upload skill with proof images -> list/detail return them -> static accessible
$ErrorActionPreference = 'Stop'

$pngPath = Join-Path $env:TEMP 'proof_test.png'
$b64 = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=='
[IO.File]::WriteAllBytes($pngPath, [Convert]::FromBase64String($b64))
"created test png: $pngPath ($((Get-Item $pngPath).Length) bytes)"

$u = 'pt_' + (Get-Random -Minimum 10000 -Maximum 99999)
$regBody = @{ username = $u; email = "$u@test.com"; password = 'Test1234!' } | ConvertTo-Json
Invoke-RestMethod -Uri 'http://localhost:8080/api/auth/register' -Method Post -ContentType 'application/json' -Body $regBody | Out-Null
$login = Invoke-RestMethod -Uri 'http://localhost:8080/api/auth/login' -Method Post -ContentType 'application/json' -Body (@{ account = $u; password = 'Test1234!' } | ConvertTo-Json)
$token = $login.token
"login ok: $u"

Add-Type -AssemblyName System.Net.Http
$handler = New-Object System.Net.Http.HttpClientHandler
$client = New-Object System.Net.Http.HttpClient($handler)
$client.DefaultRequestHeaders.Authorization = New-Object System.Net.Http.Headers.AuthenticationHeaderValue('Bearer', $token)

$form = New-Object System.Net.Http.MultipartFormDataContent
function Add-TextPart($form, $name, $value) {
    $sc = New-Object System.Net.Http.StringContent($value, [Text.Encoding]::UTF8)
    $sc.Headers.ContentType = New-Object System.Net.Http.Headers.MediaTypeHeaderValue('application/json')
    $form.Add($sc, $name)
}
Add-TextPart $form 'name' 'E2E proof images skill'
Add-TextPart $form 'description' 'verify proof image upload and display'
Add-TextPart $form 'category' 'efficiency'
Add-TextPart $form 'tags' '["test"]'
Add-TextPart $form 'version' '1.0.0'
Add-TextPart $form 'icon' ''

foreach ($i in 1..2) {
    $fc = New-Object System.Net.Http.StreamContent([IO.File]::OpenRead($pngPath))
    $fc.Headers.ContentType = New-Object System.Net.Http.Headers.MediaTypeHeaderValue('image/png')
    $form.Add($fc, 'proof_images', "proof_$i.png")
}

$resp = $client.PostAsync('http://localhost:8080/api/skills', $form).Result
$body = $resp.Content.ReadAsStringAsync().Result
"POST /api/skills -> HTTP $([int]$resp.StatusCode)"
if (-not $resp.IsSuccessStatusCode) { "BODY: $body"; exit 1 }
$created = $body | ConvertFrom-Json
$skillId = $created.data.id
"created skill id=$skillId status=$($created.status)"
"proof_images: $($created.data.proof_images -join ', ')"
if (-not $created.data.proof_images -or $created.data.proof_images.Count -ne 2) { "FAIL: expected 2 proof images"; exit 1 }

foreach ($p in $created.data.proof_images) {
    $url = 'http://localhost:8080' + $p
    $r = Invoke-WebRequest -Uri $url -Method Head -UseBasicParsing
    "static GET $p -> HTTP $($r.StatusCode)"
    if ($r.StatusCode -ne 200) { "FAIL static"; exit 1 }
}

# 5) 详情返回 proof_images（gated 技能不在市场列表，用详情+我的技能验证展示链路）
$detail = Invoke-RestMethod -Uri "http://localhost:8080/api/skills/$skillId" -Method Get
"detail id=$($detail.data.id) proof_count=$($detail.data.proof_images.Count)"
if ($detail.data.proof_images.Count -ne 2) { "FAIL: detail proof_images not returned"; exit 1 }

$mine = Invoke-RestMethod -Uri 'http://localhost:8080/api/users/me/skills' -Method Get -Headers @{ Authorization = "Bearer $token" }
$mySkill = @($mine.data) | Where-Object { $_.id -eq $skillId }
"my skills: found=$($mySkill.Count -gt 0) proof_count=$($mySkill[0].proof_images.Count)"
if (-not $mySkill -or $mySkill[0].proof_images.Count -ne 2) { "FAIL: mySkills proof_images not returned"; exit 1 }

"ALL E2E CHECKS PASSED"
