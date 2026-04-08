Describe "install.ps1" {
    It "contains required service, firewall, and ACL setup" {
        $script = Get-Content "$PSScriptRoot/install.ps1" -Raw

        $script | Should -Match "New-Service"
        $script | Should -Match "WinControlActivityService"
        $script | Should -Match "New-NetFirewallRule"
        $script | Should -Match "icacls"
        $script | Should -Match '\(OI\)\(CI\)F'
        $script | Should -Match '\(OI\)\(CI\)RX'
        $script | Should -Match "spoolRoot"
        $script | Should -Match "heartbeatRoot"
        $script | Should -Match 'Users:\(OI\)\(CI\)M'
        $script | Should -Match 'cfgRoot /grant:r "Administrators:\(OI\)\(CI\)F" "SYSTEM:\(OI\)\(CI\)F"'
        $script | Should -Not -Match 'cfgRoot /grant:r .*Users:\(OI\)\(CI\)RX'
        $script | Should -Match "sc\.exe sdset WinControlActivityService"
        $script | Should -Match ";;;SY"
        $script | Should -Match ";;;BA"
        $script | Should -Not -Match "HKLM:\\Software\\Microsoft\\Windows\\CurrentVersion\\Run"
    }
}
