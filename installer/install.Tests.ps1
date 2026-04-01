Describe "install.ps1" {
    It "contains required service, firewall, and ACL setup" {
        $script = Get-Content "$PSScriptRoot/install.ps1" -Raw

        $script | Should -Match "New-Service"
        $script | Should -Match "WinControlActivityService"
        $script | Should -Match "New-NetFirewallRule"
        $script | Should -Match "HKLM:\\Software\\Microsoft\\Windows\\CurrentVersion\\Run"
        $script | Should -Match "icacls"
        $script | Should -Match '\(OI\)\(CI\)F'
        $script | Should -Match '\(OI\)\(CI\)RX'
        $script | Should -Match "sc\.exe sdset WinControlActivityService"
        $script | Should -Match ";;;SY"
        $script | Should -Match ";;;BA"
    }
}
