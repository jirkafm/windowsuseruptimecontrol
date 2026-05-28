param(
    [string]$InstallRoot = "C:\ProgramData\Activity",
    [int]$ApiPort = 8111,
    [string]$BearerToken = "change-me",
    [ValidateSet("daily","weekly-flex")]
    [string]$QuotaMode = "daily",
    [int]$DefaultWeeklyAllowanceSec = 25200,
    [int]$UserUiPort = 0,
    [bool]$UserUiEnabled = $false
)

$ErrorActionPreference = "Stop"

if (-not ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltinRole]::Administrator)) {
    throw "Administrator rights are required."
}

$binRoot = Join-Path $InstallRoot "bin"
$cfgRoot = Join-Path $InstallRoot "config"
$logRoot = Join-Path $InstallRoot "logs"
$stateRoot = Join-Path $InstallRoot "state"

New-Item -ItemType Directory -Force -Path $binRoot, $cfgRoot, $logRoot, $stateRoot | Out-Null
Copy-Item "$PSScriptRoot\..\dist\activitysvc.exe" (Join-Path $binRoot "activitysvc.exe") -Force
Copy-Item "$PSScriptRoot\..\dist\activityhelper.exe" (Join-Path $binRoot "activityhelper.exe") -Force

icacls $binRoot /inheritance:r | Out-Null
icacls $binRoot /grant:r "Administrators:(OI)(CI)F" "SYSTEM:(OI)(CI)F" "Users:(OI)(CI)RX" | Out-Null
icacls $cfgRoot /inheritance:r | Out-Null
icacls $cfgRoot /grant:r "Administrators:(OI)(CI)F" "SYSTEM:(OI)(CI)F" | Out-Null
icacls $stateRoot /inheritance:r | Out-Null
icacls $stateRoot /grant:r "Administrators:(OI)(CI)F" "SYSTEM:(OI)(CI)F" | Out-Null

@{
    api_bind_address = "0.0.0.0"
    api_port = $ApiPort
    bearer_token = $BearerToken
    quota_mode = $QuotaMode
    default_daily_allowance_sec = 3600
    default_weekly_allowance_sec = $DefaultWeeklyAllowanceSec
    user_ui_enabled = ($UserUiEnabled -or $QuotaMode -eq "weekly-flex")
    user_ui_port = $UserUiPort
    reenforcement_delay_sec = 180
    helper_launch_cooldown_sec = 5
    warning_halfway_enabled = $true
    warning_five_min_enabled = $true
    helper_path = (Join-Path $binRoot "activityhelper.exe")
    log_level = "info"
    log_max_size_mb = 10
    log_max_backups = 10
    log_max_age_days = 365
    log_compress = $true
} | ConvertTo-Json | Set-Content (Join-Path $cfgRoot "config.json")

$serviceExe = Join-Path $binRoot "activitysvc.exe"
if (Get-Service -Name "WindowsUserUptimeControlActivityService" -ErrorAction SilentlyContinue) {
    Stop-Service -Name "WindowsUserUptimeControlActivityService" -Force
    sc.exe delete WindowsUserUptimeControlActivityService | Out-Null
    Start-Sleep -Seconds 2
}

New-Service -Name "WindowsUserUptimeControlActivityService" -BinaryPathName "`"$serviceExe`"" -DisplayName "WindowsUserUptimeControl Activity Service" -StartupType Automatic
sc.exe sdset WindowsUserUptimeControlActivityService 'D:(A;;CCLCSWRPWPDTLOCRRC;;;SY)(A;;CCDCLCSWRPWPDTLOCRSDRCWDWO;;;BA)(A;;CCLCSWLOCRRC;;;IU)(A;;CCLCSWLOCRRC;;;SU)' | Out-Null
New-NetFirewallRule -DisplayName "WindowsUserUptimeControl API" -Direction Inbound -Action Allow -Protocol TCP -LocalPort $ApiPort | Out-Null
Start-Service -Name "WindowsUserUptimeControlActivityService"
