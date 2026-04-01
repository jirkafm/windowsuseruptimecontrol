param([string]$InstallRoot = "C:\ProgramData\Activity")

$ErrorActionPreference = "Stop"

if (Get-Service -Name "WinControlActivityService" -ErrorAction SilentlyContinue) {
    Stop-Service -Name "WinControlActivityService" -Force
    sc.exe delete WinControlActivityService | Out-Null
}

Remove-ItemProperty -Path "HKLM:\Software\Microsoft\Windows\CurrentVersion\Run" -Name "WinControlActivityHelper" -ErrorAction SilentlyContinue
Get-NetFirewallRule -DisplayName "WinControl API" -ErrorAction SilentlyContinue | Remove-NetFirewallRule
Remove-Item -Path $InstallRoot -Recurse -Force -ErrorAction SilentlyContinue
