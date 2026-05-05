# Windows Validation Checklist

## Goal

Validate the `windowsuseruptimecontrol` service, helper, API, enforcement flow, and Windows-specific security behavior on a real Windows machine.

## Prerequisites

- Windows machine with Administrator access
- Two local test users, for example `John` and `Mary`
- Built binaries:
  - `dist\activitysvc.exe`
  - `dist\activityhelper.exe`
- PowerShell running as Administrator
- A chosen bearer token
- A known API port, for example `8111`

## 1. Install

- Open Administrator PowerShell in the `windowsuseruptimecontrol` project directory.
- Run:

```powershell
.\installer\install.ps1 -ApiPort 8111 -BearerToken "replace-with-test-token"
```

- Confirm service exists:

```powershell
Get-Service WindowsUserUptimeControlActivityService
```

- Expected:
  - service is present
  - status is `Running`

## 2. File And ACL Checks

- Confirm install tree exists:

```powershell
Get-ChildItem C:\ProgramData\Activity -Recurse
```

- Check ACLs on `bin` and `config`:

```powershell
icacls C:\ProgramData\Activity\bin
icacls C:\ProgramData\Activity\config
```

- Expected:
  - `Administrators` has full control
  - `SYSTEM` has full control
  - `Users` has read and execute only

- As a standard user, verify you cannot modify the config:

```powershell
Add-Content C:\ProgramData\Activity\config\config.json "x"
```

- Expected:
  - access denied

## 3. Service Startup And Logs

- Reboot the machine.
- Log in as `John`.
- Confirm service started automatically:

```powershell
Get-Service WindowsUserUptimeControlActivityService
```

- Inspect logs:

```powershell
Get-Content C:\ProgramData\Activity\logs\service.log -Tail 50
Get-Content C:\ProgramData\Activity\logs\api.log -Tail 50
```

- Expected:
  - service started without manual intervention
  - service log contains startup entries

## 4. Helper Startup And TTS

- After `John` logs in, confirm helper process exists:

```powershell
Get-Process activityhelper -ErrorAction SilentlyContinue
```

- Expected:
  - helper process is running in `John`'s session

- Trigger a manual announcement:

```powershell
Invoke-RestMethod -Method Post -Uri "http://localhost:8111/v1/announce" `
  -Headers @{ Authorization = "Bearer replace-with-test-token" } `
  -ContentType "application/json" `
  -Body '{"message":"WindowsUserUptimeControl test announcement"}'
```

- Expected:
  - API returns success
  - `John` hears the TTS announcement
  - API log contains the request

## 5. API Auth Checks

- Call health without auth:

```powershell
Invoke-WebRequest -Uri "http://localhost:8111/v1/config"
```

- Expected:
  - `401 Unauthorized`

- Call config with auth:

```powershell
Invoke-RestMethod -Uri "http://localhost:8111/v1/config" `
  -Headers @{ Authorization = "Bearer replace-with-test-token" }
```

- Expected:
  - JSON response without exposing the raw token

## 6. Active Console User Timing

- Log in as `John` and stay active for several minutes.
- Query user status:

```powershell
Invoke-RestMethod -Uri "http://localhost:8111/v1/users" `
  -Headers @{ Authorization = "Bearer replace-with-test-token" }
```

- Expected:
  - `John` appears as tracked
  - consumed time increases while `John` is the active console user

- Lock the machine or switch away from `John`.
- Wait two minutes.
- Query again.

- Expected:
  - consumed time does not continue climbing for `John` while he is not the active console user

## 7. Per-User Quota Behavior With John And Mary

- While logged in as `John`, reduce his allowance to speed validation:

```powershell
Invoke-RestMethod -Method Post -Uri "http://localhost:8111/v1/users/<john-identifier>/allowance" `
  -Headers @{ Authorization = "Bearer replace-with-test-token" } `
  -ContentType "application/json" `
  -Body '{"daily_allowance_sec":180}'
```

- Let `John` consume part of the three-minute allowance.
- Switch to `Mary`.
- Query users again.

- Expected:
  - `John` keeps his partially consumed state
  - `Mary` starts with her own full allowance
  - only the active user's time increases

## 8. Warning Milestones

- Set a short allowance for `John`, for example 120 seconds.
- Reset `John` for the day:

```powershell
Invoke-RestMethod -Method Post -Uri "http://localhost:8111/v1/users/<john-identifier>/reset-today" `
  -Headers @{ Authorization = "Bearer replace-with-test-token" }
```

- Stay active as `John`.
- Expected:
  - startup TTS announces remaining time
  - halfway TTS announces remaining time
  - near the end, last-five-minute logic should still behave sensibly for short allowances
  - final countdown speaks `10` through `1`

## 9. Hibernate And Shutdown Fallback

- On a machine with hibernation enabled, let `John` reach zero time.
- Expected:
  - countdown runs
  - machine hibernates

- On a machine without hibernation enabled, or after disabling hibernation temporarily:

```powershell
powercfg /hibernate off
```

- Repeat the zero-time flow.
- Expected:
  - hibernation attempt fails
  - system shuts down instead

## 10. Same-Day Reenforcement Delay

- Set `John`'s allowance to a short value and let it fully expire.
- Power the machine back on on the same calendar day.
- Log in as `John`.
- Expected:
  - service logs that no time remains
  - TTS warns that no time remains
  - service waits 180 seconds by default
  - then the 10-to-1 countdown runs
  - machine hibernates again

- During the 180-second delay, grant more time:

```powershell
Invoke-RestMethod -Method Post -Uri "http://localhost:8111/v1/users/<john-identifier>/adjust" `
  -Headers @{ Authorization = "Bearer replace-with-test-token" } `
  -ContentType "application/json" `
  -Body '{"delta_sec":300}'
```

- Expected:
  - reenforcement is canceled
  - `John` remains logged in

## 11. New-Day Reset

- After `John` has exhausted time, wait until the next calendar day or temporarily test with clock control in a lab environment.
- Log in again as `John`.
- Expected:
  - daily allowance resets
  - no same-day exhausted warning is issued
  - `John` receives a normal startup announcement

## 12. Helper Respawn

- While `John` is the active user, terminate the helper process:

```powershell
Stop-Process -Name activityhelper -Force
```

- Wait up to 30 seconds.
- Expected:
  - service detects the missing active helper stream
  - helper process is relaunched
  - later announcements still produce TTS

## 13. Service Stop Permissions

- As a standard non-admin user, try:

```powershell
Stop-Service WindowsUserUptimeControlActivityService
sc.exe stop WindowsUserUptimeControlActivityService
taskkill /IM activitysvc.exe /F
```

- Expected:
  - all attempts fail

- As an Administrator, try:

```powershell
Stop-Service WindowsUserUptimeControlActivityService
Start-Service WindowsUserUptimeControlActivityService
```

- Expected:
  - stop and start succeed

## 14. Network Reachability

- From another machine on the same network, call:

```powershell
Invoke-RestMethod -Uri "http://<target-host>:8111/v1/health"
```

- Expected:
  - endpoint is reachable

- Repeat with an authenticated endpoint:

```powershell
Invoke-RestMethod -Uri "http://<target-host>:8111/v1/users" `
  -Headers @{ Authorization = "Bearer replace-with-test-token" }
```

- Expected:
  - management endpoint works remotely

## 15. Pass Criteria

- Service starts automatically after install and reboot
- Helper starts for the logged-in user and TTS is audible
- Per-user quotas are tracked separately
- Only the active console user consumes time
- Countdown and enforcement execute correctly
- Same-day reenforcement delay works
- New-day reset works
- Standard users cannot stop the service or modify protected files
- Administrators can manage and uninstall the system
