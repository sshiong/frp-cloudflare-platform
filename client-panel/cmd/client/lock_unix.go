//go:build !windows

package main

import (
	"os"
	"syscall"
)

// platformFileLock Unix 平台文件锁（flock）
func platformFileLock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

// platformFileUnlock Unix 平台文件解锁
func platformFileUnlock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
