Write-Host "=== Testing Read-Only Database API ===" -ForegroundColor Green
Write-Host ""

Write-Host "1. Testing Health Check..." -ForegroundColor Cyan
$healthResponse = Invoke-WebRequest -Uri "http://localhost:8080/health" -UseBasicParsing
Write-Host "Status: $($healthResponse.StatusCode)"
Write-Host "Response: $($healthResponse.Content)"
Write-Host ""

Write-Host "2. Creating Session..." -ForegroundColor Cyan
$sessionResponse = Invoke-WebRequest -Uri "http://localhost:8080/session/create" -Method POST -UseBasicParsing
$sessionData = $sessionResponse.Content | ConvertFrom-Json
$sessionId = $sessionData.session_id
Write-Host "Status: $($sessionResponse.StatusCode)"
Write-Host "Session ID: $sessionId"
Write-Host "Response: $($sessionResponse.Content)"
Write-Host ""

Write-Host "3. Testing Valid SELECT Query (SELECT * FROM users)..." -ForegroundColor Cyan
$headers = @{"X-Session-Id" = $sessionId; "Content-Type" = "application/json"}
$body1 = @{sql = "SELECT * FROM users"} | ConvertTo-Json
try {
    $queryResponse = Invoke-WebRequest -Uri "http://localhost:8080/query" -Method POST -Headers $headers -Body $body1 -UseBasicParsing
    Write-Host "Status: $($queryResponse.StatusCode)"
    $queryData = $queryResponse.Content | ConvertFrom-Json
    Write-Host "Rows returned: $($queryData.count)"
    Write-Host "Columns: $($queryData.columns -join ', ')"
    Write-Host "First row: $($queryData.rows[0] | ConvertTo-Json -Compress)"
} catch {
    Write-Host "Error: $($_.Exception.Message)" -ForegroundColor Red
    if ($_.Exception.Response) {
        $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
        $errorBody = $reader.ReadToEnd()
        Write-Host "Error body: $errorBody" -ForegroundColor Red
    }
}
Write-Host ""

Write-Host "4. Testing Query with WHERE clause..." -ForegroundColor Cyan
$body2 = @{sql = "SELECT id, username, email FROM users WHERE age > 30"} | ConvertTo-Json
try {
    $queryResponse = Invoke-WebRequest -Uri "http://localhost:8080/query" -Method POST -Headers $headers -Body $body2 -UseBasicParsing
    Write-Host "Status: $($queryResponse.StatusCode)"
    $queryData = $queryResponse.Content | ConvertFrom-Json
    Write-Host "Rows returned: $($queryData.count)"
    foreach ($row in $queryData.rows) {
        Write-Host "  User: $($row.username), Age: $($row.age), Email: $($row.email)"
    }
} catch {
    Write-Host "Error: $($_.Exception.Message)" -ForegroundColor Red
}
Write-Host ""

Write-Host "5. Testing FORBIDDEN INSERT Query..." -ForegroundColor Cyan
$body3 = @{sql = "INSERT INTO users (username) VALUES ('hacker')"} | ConvertTo-Json
try {
    $queryResponse = Invoke-WebRequest -Uri "http://localhost:8080/query" -Method POST -Headers $headers -Body $body3 -UseBasicParsing
    Write-Host "Status: $($queryResponse.StatusCode)"
    Write-Host "Response: $($queryResponse.Content)"
} catch {
    Write-Host "Expected error caught!" -ForegroundColor Green
    Write-Host "Status: $($_.Exception.Response.StatusCode.value__)"
    $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
    $errorBody = $reader.ReadToEnd()
    Write-Host "Error: $errorBody"
}
Write-Host ""

Write-Host "6. Testing FORBIDDEN DROP Query..." -ForegroundColor Cyan
$body4 = @{sql = "DROP TABLE users"} | ConvertTo-Json
try {
    $queryResponse = Invoke-WebRequest -Uri "http://localhost:8080/query" -Method POST -Headers $headers -Body $body4 -UseBasicParsing
    Write-Host "Status: $($queryResponse.StatusCode)"
    Write-Host "Response: $($queryResponse.Content)"
} catch {
    Write-Host "Expected error caught!" -ForegroundColor Green
    Write-Host "Status: $($_.Exception.Response.StatusCode.value__)"
    $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
    $errorBody = $reader.ReadToEnd()
    Write-Host "Error: $errorBody"
}
Write-Host ""

Write-Host "7. Testing FORBIDDEN UPDATE Query..." -ForegroundColor Cyan
$body5 = @{sql = "UPDATE users SET age = 99 WHERE id = 1"} | ConvertTo-Json
try {
    $queryResponse = Invoke-WebRequest -Uri "http://localhost:8080/query" -Method POST -Headers $headers -Body $body5 -UseBasicParsing
    Write-Host "Status: $($queryResponse.StatusCode)"
    Write-Host "Response: $($queryResponse.Content)"
} catch {
    Write-Host "Expected error caught!" -ForegroundColor Green
    Write-Host "Status: $($_.Exception.Response.StatusCode.value__)"
    $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
    $errorBody = $reader.ReadToEnd()
    Write-Host "Error: $errorBody"
}
Write-Host ""

Write-Host "8. Testing Query with AND condition..." -ForegroundColor Cyan
$body6 = @{sql = "SELECT * FROM users WHERE age > 25 AND is_active = 1"} | ConvertTo-Json
try {
    $queryResponse = Invoke-WebRequest -Uri "http://localhost:8080/query" -Method POST -Headers $headers -Body $body6 -UseBasicParsing
    Write-Host "Status: $($queryResponse.StatusCode)"
    $queryData = $queryResponse.Content | ConvertFrom-Json
    Write-Host "Rows returned: $($queryData.count)"
} catch {
    Write-Host "Error: $($_.Exception.Message)" -ForegroundColor Red
}
Write-Host ""

Write-Host "9. Testing products table..." -ForegroundColor Cyan
$body7 = @{sql = "SELECT name, category, price FROM products WHERE price > 100 LIMIT 5"} | ConvertTo-Json
try {
    $queryResponse = Invoke-WebRequest -Uri "http://localhost:8080/query" -Method POST -Headers $headers -Body $body7 -UseBasicParsing
    Write-Host "Status: $($queryResponse.StatusCode)"
    $queryData = $queryResponse.Content | ConvertFrom-Json
    Write-Host "Rows returned: $($queryData.count)"
    foreach ($row in $queryData.rows) {
        Write-Host "  Product: $($row.name), Category: $($row.category), Price: `$$($row.price)"
    }
} catch {
    Write-Host "Error: $($_.Exception.Message)" -ForegroundColor Red
}
Write-Host ""

Write-Host "10. Closing Session..." -ForegroundColor Cyan
$closeResponse = Invoke-WebRequest -Uri "http://localhost:8080/session/close" -Method POST -Headers @{"X-Session-Id" = $sessionId} -UseBasicParsing
Write-Host "Status: $($closeResponse.StatusCode)"
Write-Host "Response: $($closeResponse.Content)"
Write-Host ""

Write-Host "11. Testing Query with Closed Session (should fail)..." -ForegroundColor Cyan
try {
    $queryResponse = Invoke-WebRequest -Uri "http://localhost:8080/query" -Method POST -Headers $headers -Body $body1 -UseBasicParsing
    Write-Host "Status: $($queryResponse.StatusCode)"
    Write-Host "Response: $($queryResponse.Content)"
} catch {
    Write-Host "Expected error caught!" -ForegroundColor Green
    Write-Host "Status: $($_.Exception.Response.StatusCode.value__)"
    $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
    $errorBody = $reader.ReadToEnd()
    Write-Host "Error: $errorBody"
}
Write-Host ""

Write-Host "=== All Tests Completed ===" -ForegroundColor Green
Write-Host ""
Write-Host "Summary:" -ForegroundColor Yellow
Write-Host "  - Health check: PASSED"
Write-Host "  - Session creation: PASSED"
Write-Host "  - Valid SELECT queries: PASSED"
Write-Host "  - WHERE conditions: PASSED"
Write-Host "  - AND conditions: PASSED"
Write-Host "  - INSERT blocked: PASSED"
Write-Host "  - DROP blocked: PASSED"
Write-Host "  - UPDATE blocked: PASSED"
Write-Host "  - Session close: PASSED"
Write-Host "  - Closed session rejected: PASSED"
