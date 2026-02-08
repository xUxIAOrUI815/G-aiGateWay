# 1. 强制设置终端为 UTF8 编码
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8

$url = "http://localhost:8080/v1/chat/completions"
$headers = @{"Content-Type"="application/json"}

# 定义测试问题
$q1 = '{"model":"deepseek-chat","messages":[{"role":"user","content":"如何学习Go语言？"}],"stream":false}'
$q1_same = '{"model":"deepseek-chat","messages":[{"role":"user","content":"如何学习Go语言？"}],"stream":false}'
$q1_semantic = '{"model":"deepseek-chat","messages":[{"role":"user","content":"我想学Go，我该怎么做"}],"stream":false}'
function Test-Request($body, $label) {
    Write-Host "`n>>> [$label]" -ForegroundColor Cyan
    $start = Get-Date
    
    try {
        # 使用 -UseBasicParsing 避免 IE 引擎弹窗
        $r = Invoke-WebRequest -Uri $url -Method Post -Headers $headers -Body $body -UseBasicParsing -TimeoutSec 60
        $end = Get-Date
        $duration = ($end - $start).TotalMilliseconds
        
        # 提取 Header
        $cacheStatus = $r.Headers["X-Cache-Hit"]
        if ($null -eq $cacheStatus) { $cacheStatus = "false (Miss)" }

        # 处理内容摘要：去掉换行符并截取前 60 个字符
        $cleanContent = $r.Content -replace "[\r\n]", " "
        if ($cleanContent.Length -gt 60) {
            $summary = $cleanContent.Substring(0, 60) + "..."
        } else {
            $summary = $cleanContent
        }

        Write-Host "状态码: " $r.StatusCode
        Write-Host "耗时:   " ([math]::Round($duration, 2)) "ms"
        Write-Host "缓存命中:" $cacheStatus -ForegroundColor Yellow
        Write-Host "内容摘要:" $summary
    }
    catch {
        Write-Host "请求失败: " $_.Exception.Message -ForegroundColor Red
    }
}

# --- 执行测试场景 ---

# 场景 1：第一次请求（冷启动）
Test-Request $q1 "冷启动：第一次提问"

# 场景 2：完全相同的提问
Start-Sleep -Seconds 2 # 给后台异步存入缓存留出时间
Test-Request $q1_same "L1 命中：完全相同的提问 (Hash 匹配)"

# 场景 3：语义相近的提问
Start-Sleep -Seconds 1
Test-Request $q1_semantic "L2 命中：语义相近的提问 (向量匹配)"

# 场景 4：高并发限流测试
Write-Host "`n>>> 场景 4：高并发限流测试 (预期触发 429)" -ForegroundColor Red
for($i=1; $i -le 6; $i++){
    Write-Host "正在发送第 $i 次快速请求..."
    try { 
        $resp = Invoke-WebRequest -Uri $url -Method Post -Headers $headers -Body $q1 -UseBasicParsing -ErrorAction Stop
        Write-Host "结果: 成功 (Status $($resp.StatusCode))"
    } 
    catch { 
        Write-Host "结果: 已拦截 ($($_.Exception.Message))" -ForegroundColor Gray 
    }
}

Write-Host "`n--- 测试结束 ---" -ForegroundColor Cyan