package app

import (
	"crypto/sha256"
	"fmt"
	"syscall"
	"unsafe"
)

func instanceLock(root string) (func(), error) {
	kernel := syscall.NewLazyDLL("kernel32.dll")
	create := kernel.NewProc("CreateMutexW")
	closeHandle := kernel.NewProc("CloseHandle")
	name, e := syscall.UTF16PtrFromString(fmt.Sprintf(`Local\GeoCheats-%x`, sha256.Sum256([]byte(root))))
	if e != nil {
		return nil, e
	}
	handle, _, err := create.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if handle == 0 {
		return nil, err
	}
	if err == syscall.Errno(183) {
		_, _, _ = closeHandle.Call(handle)
		return nil, errAlreadyRunning
	}
	return func() { _, _, _ = closeHandle.Call(handle) }, nil
}
