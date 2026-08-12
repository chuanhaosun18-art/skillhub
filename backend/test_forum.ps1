$ErrorActionPreference = "Stop"
$enc = New-Object System.Text.UTF8Encoding($false)

# 1) register
$u = "forum_test_" + (Get-Random -Max 99999)
$body = @{ username = $u; password = "test123456" } | ConvertTo-Json
[System.IO.File]::WriteAllText("$env:TEMP\reg.json", $body, $enc)
$reg = curl.exe -s -m 15 -X POST "http://127.0.0.1:8080/api/auth/register" -H "Content-Type: application/json" --data-binary "@$env:TEMP\reg.json"
$obj = $reg | ConvertFrom-Json
if (-not $obj.token) { Write-Host "REGISTER FAIL: $reg"; exit 1 }
Write-Host "REGISTER OK: $u"
$h = @{ Authorization = "Bearer $($obj.token)" }

# 2) create topic (help)
$t = @{ title = "how to rewrite research exp into product resume"; content = "tried two versions, hr said not readable"; category = "help" } | ConvertTo-Json
[System.IO.File]::WriteAllText("$env:TEMP\post.json", $t, $enc)
$post = curl.exe -s -m 15 -X POST "http://127.0.0.1:8080/api/forum/topics" -H "Content-Type: application/json" -H "Authorization: Bearer $($obj.token)" --data-binary "@$env:TEMP\post.json"
$po = $post | ConvertFrom-Json
if (-not $po.data.id) { Write-Host "POST FAIL: $post"; exit 1 }
$tid = $po.data.id
Write-Host "POST OK: topic_id=$tid"

# 3) create topic (looking_for)
$t2 = @{ title = "looking for anyone who made flutter skill"; content = "none found in skill hub"; category = "looking_for" } | ConvertTo-Json
[System.IO.File]::WriteAllText("$env:TEMP\post2.json", $t2, $enc)
$post2 = curl.exe -s -m 15 -X POST "http://127.0.0.1:8080/api/forum/topics" -H "Content-Type: application/json" -H "Authorization: Bearer $($obj.token)" --data-binary "@$env:TEMP\post2.json"
$po2 = $post2 | ConvertFrom-Json
Write-Host "POST2 OK: topic_id=$($po2.data.id)"

# 4) list
$list = curl.exe -s -m 15 "http://127.0.0.1:8080/api/forum/topics"
Write-Host "LIST: $list"

# 5) list with keyword=resume
$lk = curl.exe -s -m 15 "http://127.0.0.1:8080/api/forum/topics?keyword=resume"
Write-Host "LIST keyword: $lk"

# 6) list by category=looking_for
$lc = curl.exe -s -m 15 "http://127.0.0.1:8080/api/forum/topics?category=looking_for"
Write-Host "LIST cat: $lc"

# 7) detail
$det = curl.exe -s -m 15 "http://127.0.0.1:8080/api/forum/topics/$tid"
Write-Host "DETAIL: $det"

# 8) reply
$r = @{ content = "same pitfall here: write outcomes first, then story line" } | ConvertTo-Json
[System.IO.File]::WriteAllText("$env:TEMP\rep.json", $r, $enc)
$rep = curl.exe -s -m 15 -X POST "http://127.0.0.1:8080/api/forum/topics/$tid/replies" -H "Content-Type: application/json" -H "Authorization: Bearer $($obj.token)" --data-binary "@$env:TEMP\rep.json"
Write-Host "REPLY: $rep"

# 9) detail again (reply_count + view_count)
$det2 = curl.exe -s -m 15 "http://127.0.0.1:8080/api/forum/topics/$tid"
Write-Host "DETAIL2: $det2"

# 10) anonymous post -> expect 401
$anon = curl.exe -s -m 15 -o NUL -w "%{http_code}" -X POST "http://127.0.0.1:8080/api/forum/topics" -H "Content-Type: application/json" --data-binary "@$env:TEMP\post.json"
Write-Host "ANON POST status(expect 401): $anon"
