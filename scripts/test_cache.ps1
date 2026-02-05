# 1. 设置环境
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$url = "http://localhost:8080/v1/chat/completions"
$headers = @{"Content-Type"="application/json"}
$body = '{"model":"deepseek-chat","messages":[{"role":"user","content":"你好"}],"stream":false}'

Write-Host "--- 发起第一次请求 (预期: 转发给上游) ---" -ForegroundColor Cyan
$resp1 = Invoke-WebRequest -Uri $url -Method Post -Headers $headers -Body $body -UseBasicParsing
Write-Host "Cache Hit: " $resp1.Headers["X-Cache-Hit"]
Write-Host "Response Body: " $resp1.Content

Write-Host "`n--- 发起第二次请求 (预期: 命中本地缓存) ---" -ForegroundColor Green
$resp2 = Invoke-WebRequest -Uri $url -Method Post -Headers $headers -Body $body -UseBasicParsing
Write-Host "Cache Hit: " $resp2.Headers["X-Cache-Hit"]
Write-Host "Response Body: " $resp2.Content