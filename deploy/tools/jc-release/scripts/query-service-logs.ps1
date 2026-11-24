[CmdletBinding()]
param(
    [string]$BaseUrl = $env:JC_RELEASE_URL,
    [string]$AgentId,
    [Parameter(Mandatory = $true)]
    [ValidateSet('backend', 'frontend', 'admin', 'tgbot', 'redis', 'caddy', 'mysql', 'hardhat', 'agent')]
    [string]$Target,
    [ValidateRange(1, 2000)]
    [int]$Lines = 200,
    [ValidateRange(1, 604800)]
    [int]$SinceSeconds = 3600,
    [ValidateRange(10, 300)]
    [int]$TimeoutSeconds = 120
)

$ErrorActionPreference = 'Stop'
$token = $env:JC_RELEASE_READ_TOKEN
if ([string]::IsNullOrWhiteSpace($BaseUrl)) {
    throw 'BaseUrl or JC_RELEASE_URL is required.'
}
if ([string]::IsNullOrWhiteSpace($token)) {
    throw 'JC_RELEASE_READ_TOKEN is required.'
}

$base = $BaseUrl.TrimEnd('/')
$headers = @{ Authorization = "Bearer $token" }
$catalog = (Invoke-RestMethod -Method Get -Uri "$base/api/v1/service-logs" -Headers $headers).data

if ([string]::IsNullOrWhiteSpace($AgentId)) {
    if ($catalog.agents.Count -eq 1) {
        $AgentId = $catalog.agents[0].id
    }
    elseif ($catalog.agents.Count -eq 0) {
        throw 'No registered agents are available.'
    }
    else {
        $choices = ($catalog.agents | ForEach-Object { "$($_.hostname)=$($_.id)" }) -join ', '
        throw "Multiple agents are available; pass -AgentId. $choices"
    }
}

if ($catalog.targets -notcontains $Target) {
    throw "Target '$Target' is not enabled by the release server."
}

$payload = @{
    agent_id     = $AgentId
    target       = $Target
    lines        = $Lines
    since_seconds = $SinceSeconds
} | ConvertTo-Json

$queued = (Invoke-RestMethod -Method Post -Uri "$base/api/v1/service-logs" `
    -Headers $headers -ContentType 'application/json' -Body $payload).data

$deadline = (Get-Date).AddSeconds($TimeoutSeconds)
do {
    Start-Sleep -Seconds 2
    $result = (Invoke-RestMethod -Method Get -Uri "$base/api/v1/service-logs/$($queued.id)" -Headers $headers).data
    if ($result.status -in @('success', 'failed')) {
        Write-Output "query_id=$($result.id) target=$($result.component) status=$($result.status)"
        if (-not [string]::IsNullOrEmpty($result.log)) {
            Write-Output $result.log
        }
        if ($result.status -eq 'failed') {
            throw "Service log query failed: $($result.error)"
        }
        return
    }
} while ((Get-Date) -lt $deadline)

throw "Timed out waiting for service log query $($queued.id)."
