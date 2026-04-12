param([string]$InstallRoot = "C:\ProgramData\Activity")

$ErrorActionPreference = "Stop"

if (Get-Service -Name "WindowsUserUptimeControlActivityService" -ErrorAction SilentlyContinue) {
    Stop-Service -Name "WindowsUserUptimeControlActivityService" -Force
    sc.exe delete WindowsUserUptimeControlActivityService | Out-Null
}

Remove-ItemProperty -Path "HKLM:\Software\Microsoft\Windows\CurrentVersion\Run" -Name "WindowsUserUptimeControlActivityHelper" -ErrorAction SilentlyContinue
Get-NetFirewallRule -DisplayName "WindowsUserUptimeControl API" -ErrorAction SilentlyContinue | Remove-NetFirewallRule
Remove-Item -Path $InstallRoot -Recurse -Force -ErrorAction SilentlyContinue
