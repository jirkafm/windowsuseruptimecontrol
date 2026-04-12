//go:build windows

package session

import (
	"context"
	"fmt"
	"os/user"
	"syscall"
	"unsafe"

	"windowsuseruptimecontrol/internal/model"
)

type Detector struct{}

func (Detector) ActiveUser(context.Context) (model.ActiveUser, bool, error) {
	sessionID := wtsGetActiveConsoleSessionID()
	if sessionID == 0xFFFFFFFF {
		return model.ActiveUser{}, false, nil
	}

	username, err := querySessionString(sessionID, wtsUserName)
	if err != nil {
		return model.ActiveUser{}, false, err
	}
	if username == "" {
		return model.ActiveUser{}, false, nil
	}

	domain, err := querySessionString(sessionID, wtsDomainName)
	if err != nil {
		return model.ActiveUser{}, false, err
	}

	qualified := username
	if domain != "" {
		qualified = domain + `\` + username
	}

	current, err := user.Lookup(qualified)
	if err != nil {
		return model.ActiveUser{
			SessionID: sessionID,
			Username:  qualified,
			UserSID:   qualified,
		}, true, nil
	}

	return model.ActiveUser{
		SessionID: sessionID,
		Username:  qualified,
		UserSID:   current.Uid,
	}, true, nil
}

const (
	wtsUserName   = 5
	wtsDomainName = 7
)

var (
	modKernel32               = syscall.NewLazyDLL("kernel32.dll")
	procWTSGetActiveConsoleID = modKernel32.NewProc("WTSGetActiveConsoleSessionId")
	modWTSAPI32               = syscall.NewLazyDLL("wtsapi32.dll")
	procWTSQuerySessionInfoW  = modWTSAPI32.NewProc("WTSQuerySessionInformationW")
	procWTSFreeMemory         = modWTSAPI32.NewProc("WTSFreeMemory")
)

func wtsGetActiveConsoleSessionID() uint32 {
	r1, _, _ := procWTSGetActiveConsoleID.Call()
	return uint32(r1)
}

func querySessionString(sessionID uint32, infoClass uint32) (string, error) {
	var buffer uintptr
	var bytesReturned uint32
	r1, _, err := procWTSQuerySessionInfoW.Call(
		0,
		uintptr(sessionID),
		uintptr(infoClass),
		uintptr(unsafe.Pointer(&buffer)),
		uintptr(unsafe.Pointer(&bytesReturned)),
	)
	if r1 == 0 {
		return "", fmt.Errorf("WTSQuerySessionInformationW failed: %w", err)
	}
	defer procWTSFreeMemory.Call(buffer)

	if buffer == 0 || bytesReturned < 2 {
		return "", nil
	}
	return syscall.UTF16ToString((*[1 << 20]uint16)(unsafe.Pointer(buffer))[:]), nil
}
