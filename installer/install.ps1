param(
    [string]$InstallRoot = "C:\ProgramData\Activity",
    [int]$ApiPort = 8111,
    [string]$BearerToken = "change-me"
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
icacls $cfgRoot /grant:r "Administrators:(OI)(CI)F" "SYSTEM:(OI)(CI)F" "Users:(OI)(CI)RX" | Out-Null

@{
    api_bind_address = "0.0.0.0"
    api_port = $ApiPort
    bearer_token = $BearerToken
    default_daily_allowance_sec = 3600
    reenforcement_delay_sec = 180
    warning_halfway_enabled = $true
    warning_five_min_enabled = $true
    helper_path = (Join-Path $binRoot "activityhelper.exe")
    log_level = "info"
} | ConvertTo-Json | Set-Content (Join-Path $cfgRoot "config.json")

$serviceExe = Join-Path $binRoot "activitysvc.exe"
if (Get-Service -Name "WinControlActivityService" -ErrorAction SilentlyContinue) {
    Stop-Service -Name "WinControlActivityService" -Force
    sc.exe delete WinControlActivityService | Out-Null
    Start-Sleep -Seconds 2
}

New-Service -Name "WinControlActivityService" -BinaryPathName "`"$serviceExe`"" -DisplayName "WinControl Activity Service" -StartupType Automatic
sc.exe sdset WinControlActivityService 'D:(A;;CCLCSWRPWPDTLOCRRC;;;SY)(A;;CCDCLCSWRPWPDTLOCRSDRCWDWO;;;BA)(A;;CCLCSWLOCRRC;;;IU)(A;;CCLCSWLOCRRC;;;SU)' | Out-Null
New-ItemProperty -Path "HKLM:\Software\Microsoft\Windows\CurrentVersion\Run" -Name "WinControlActivityHelper" -Value (Join-Path $binRoot "activityhelper.exe") -PropertyType String -Force | Out-Null
New-NetFirewallRule -DisplayName "WinControl API" -Direction Inbound -Action Allow -Protocol TCP -LocalPort $ApiPort | Out-Null
Start-Service -Name "WinControlActivityService"
