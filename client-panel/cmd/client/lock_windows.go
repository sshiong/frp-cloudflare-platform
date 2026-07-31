//go:build windows

package main

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32        = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx  = kernel32.NewProc("LockFileEx")
	procUnlockFile  = kernel32.NewProc("UnlockFile")
)

const (
	LOCKFILE_EXCLUSIVE_LOCK   = 0x00000002
	LOCKFILE_FAIL_IMMEDIATELY = 0x00000001
)

// platformFileLock Windows 平台文件锁
func platformFileLock(f *os.File) error {
	handle := syscall.Handle(f.Fd())

	var overlapped syscall.Overlapped
	ret, _, err := procLockFileEx.Call(
		uintptr(handle),
		uintptr(LOCKFILE_EXCLUSIVE_LOCK|LOCKFILE_FAIL_IMMEDIATELY),
		uintptr(0),
		uintptr(1),
		uintptr(0),
		uintptr(unsafe.Pointer(&overlapped)),
	)
	if ret == 0 {
		return err
	}
	return nil
}

// platformFileUnlock Windows 平台文件解锁
func platformFileUnlock(f *os.File) error {
	handle := syscall.Handle(f.Fd())

	var overlapped syscall.Overlapped
	ret, _, err := procUnlockFile.Call(
		uintptr(handle),
		uintptr(0),
		uintptr(0),
		uintptr(1),
		uintptr(0),
		uintptr(unsafe.Pointer(&overlapped)),
	)
	if ret == 0 {
		return err
	}
	return nil
}
