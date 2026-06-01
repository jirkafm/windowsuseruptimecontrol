Describe "install.ps1" {
    It "contains required service, firewall, and ACL setup" {
        $script = Get-Content "$PSScriptRoot/install.ps1" -Raw

        $script | Should -Match "New-Service"
        $script | Should -Match "WindowsUserUptimeControlActivityService"
        $script | Should -Match "New-NetFirewallRule"
        $script | Should -Match "icacls"
        $script | Should -Match '\(OI\)\(CI\)F'
        $script | Should -Match '\(OI\)\(CI\)RX'
        $script | Should -Not -Match 'Users:\(OI\)\(CI\)M'
        $script | Should -Match 'cfgRoot /grant:r "Administrators:\(OI\)\(CI\)F" "SYSTEM:\(OI\)\(CI\)F"'
        $script | Should -Match 'stateRoot /grant:r "Administrators:\(OI\)\(CI\)F" "SYSTEM:\(OI\)\(CI\)F"'
        $script | Should -Not -Match 'cfgRoot /grant:r .*Users:\(OI\)\(CI\)RX'
        $script | Should -Match "sc\.exe sdset WindowsUserUptimeControlActivityService"
        $script | Should -Match ";;;SY"
        $script | Should -Match ";;;BA"
        $script | Should -Not -Match "HKLM:\\Software\\Microsoft\\Windows\\CurrentVersion\\Run"
        $script | Should -Match '\[ValidateSet\("daily","weekly-flex","scheduled-days"\)\]'
        $script | Should -Match '\$QuotaMode = "scheduled-days"'
        $script | Should -Match '\[ValidateSet\("en","cs","en-US","cs-CZ"\)\]'
        $script | Should -Match '\$Language = "en"'
        $script | Should -Match 'language = \$Language'
        $script | Should -Match '\$EnabledWeekdays = @\(\$true, \$true, \$true, \$true, \$true, \$false, \$false\)'
        $script | Should -Match 'EnabledWeekdays must contain 7 boolean values ordered Monday through Sunday'
        $script | Should -Match 'enabled_weekdays = \$EnabledWeekdays'
        $script | Should -Match 'default_weekly_allowance_sec = \$DefaultWeeklyAllowanceSec'
        $script | Should -Match 'user_ui_port = \$UserUiPort'
    }
}
