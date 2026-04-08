//go:build windows

package helper

import (
	"context"
	"fmt"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"wincontrol/internal/helperstatus"
)

func (l Launcher) EnsureRunning(ctx context.Context, sessionID uint32, userSID string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if l.HelperPath == "" || userSID == "" {
		return nil
	}

	store := helperstatus.New(l.HeartbeatRoot)
	hb, err := store.Load(userSID)
	if err == nil && !shouldLaunch(time.Now(), hb, sessionID) {
		return nil
	}

	token, err := queryUserToken(sessionID)
	if err != nil {
		return err
	}
	defer token.Close()

	primary, err := duplicatePrimaryToken(token)
	if err != nil {
		return err
	}
	defer primary.Close()

	dir := filepath.Dir(l.HelperPath)
	commandLine := fmt.Sprintf("\"%s\" --session-id %d", l.HelperPath, sessionID)
	commandLinePtr, err := windows.UTF16PtrFromString(commandLine)
	if err != nil {
		return err
	}
	dirPtr, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return err
	}

	settings := defaultLaunchSettings()
	si := newStartupInfo(settings)
	pi := new(windows.ProcessInformation)
	flags := uint32(windows.CREATE_UNICODE_ENVIRONMENT | windows.CREATE_NEW_PROCESS_GROUP)
	if settings.NoConsoleWindow {
		flags |= windows.CREATE_NO_WINDOW
	}
	if err := windows.CreateProcessAsUser(
		primary,
		nil,
		commandLinePtr,
		nil,
		nil,
		false,
		flags,
		nil,
		dirPtr,
		si,
		pi,
	); err != nil {
		return err
	}
	defer windows.CloseHandle(pi.Process)
	defer windows.CloseHandle(pi.Thread)

	return nil
}

func queryUserToken(sessionID uint32) (windows.Token, error) {
	var token windows.Token
	if err := windows.WTSQueryUserToken(sessionID, &token); err != nil {
		return 0, err
	}
	return token, nil
}

func duplicatePrimaryToken(token windows.Token) (windows.Token, error) {
	var primary windows.Token
	if err := windows.DuplicateTokenEx(
		token,
		windows.MAXIMUM_ALLOWED,
		nil,
		windows.SecurityImpersonation,
		windows.TokenPrimary,
		&primary,
	); err != nil {
		return 0, err
	}
	return primary, nil
}

func newStartupInfo(settings launchSettings) *windows.StartupInfo {
	si := &windows.StartupInfo{}
	si.Cb = uint32(unsafe.Sizeof(*si))
	si.Desktop = syscall.StringToUTF16Ptr("winsta0\\default")
	if settings.HideWindow {
		si.Flags |= windows.STARTF_USESHOWWINDOW
		si.ShowWindow = windows.SW_HIDE
	}
	return si
}
