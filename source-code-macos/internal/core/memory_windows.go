package core

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var kernel = syscall.NewLazyDLL("kernel32.dll")
var user = syscall.NewLazyDLL("user32.dll")
var openProcess = kernel.NewProc("OpenProcess")
var closeHandle = kernel.NewProc("CloseHandle")
var readMemory = kernel.NewProc("ReadProcessMemory")
var writeMemory = kernel.NewProc("WriteProcessMemory")
var protectMemory = kernel.NewProc("VirtualProtectEx")
var flushCache = kernel.NewProc("FlushInstructionCache")
var snapshot = kernel.NewProc("CreateToolhelp32Snapshot")
var processFirst = kernel.NewProc("Process32FirstW")
var processNext = kernel.NewProc("Process32NextW")
var moduleFirst = kernel.NewProc("Module32FirstW")
var threadFirst = kernel.NewProc("Thread32First")
var threadNext = kernel.NewProc("Thread32Next")
var openThread = kernel.NewProc("OpenThread")
var suspendThread = kernel.NewProc("SuspendThread")
var resumeThread = kernel.NewProc("ResumeThread")
var queryPath = kernel.NewProc("QueryFullProcessImageNameW")
var waitProcess = kernel.NewProc("WaitForSingleObject")
var asyncKey = user.NewProc("GetAsyncKeyState")
var foreground = user.NewProc("GetForegroundWindow")
var windowPID = user.NewProc("GetWindowThreadProcessId")

type processEntry struct {
	Size, Usage, PID        uint32
	Heap                    uintptr
	Module, Threads, Parent uint32
	Priority                int32
	Flags                   uint32
	Name                    [260]uint16
}
type moduleEntry struct {
	Size, Module, PID, GlobalUsage, Usage uint32
	Base                                  uintptr
	ImageSize                             uint32
	Handle                                uintptr
	Name                                  [256]uint16
	Path                                  [260]uint16
}
type threadEntry struct {
	Size, Usage, TID, PID uint32
	Base, Delta           int32
	Flags                 uint32
}
type winMemory struct {
	h    uintptr
	info ProcessInfo
}

func winerr(action string, e error) error { return fmt.Errorf("%s: %w", action, e) }
func FindProcesses(path string) ([]ProcessInfo, error) {
	var result []ProcessInfo
	snap, _, e := snapshot.Call(2, 0)
	if snap == ^uintptr(0) {
		return nil, e
	}
	defer closeHandle.Call(snap)
	pe := processEntry{}
	pe.Size = uint32(unsafe.Sizeof(pe))
	ok, _, _ := processFirst.Call(snap, uintptr(unsafe.Pointer(&pe)))
	for ok != 0 {
		if strings.EqualFold(syscall.UTF16ToString(pe.Name[:]), "GeometryDash.exe") {
			h, _, _ := openProcess.Call(0x1000, 0, uintptr(pe.PID))
			if h != 0 {
				buf := make([]uint16, 32768)
				n := uint32(len(buf))
				r, _, _ := queryPath.Call(h, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)))
				closeHandle.Call(h)
				found := syscall.UTF16ToString(buf[:n])
				if r != 0 && strings.EqualFold(filepath.Clean(found), filepath.Clean(path)) {
					ms, _, _ := snapshot.Call(0x8|0x10, uintptr(pe.PID))
					if ms != ^uintptr(0) {
						me := moduleEntry{}
						me.Size = uint32(unsafe.Sizeof(me))
						r, _, _ := moduleFirst.Call(ms, uintptr(unsafe.Pointer(&me)))
						if r != 0 {
							result = append(result, ProcessInfo{int(pe.PID), found, uint64(me.Base), uint64(me.ImageSize)})
						}
						closeHandle.Call(ms)
					}
				}
			}
		}
		ok, _, _ = processNext.Call(snap, uintptr(unsafe.Pointer(&pe)))
	}
	return result, nil
}
func OpenMemory(info ProcessInfo) (Memory, error) {
	h, _, e := openProcess.Call(0x0400|0x0008|0x0010|0x0020|0x100000, 0, uintptr(info.PID))
	if h == 0 {
		return nil, winerr("attach failed", e)
	}
	return &winMemory{h, info}, nil
}
func (m *winMemory) Info() ProcessInfo { return m.info }
func (m *winMemory) Read(a uint64, n int) ([]byte, error) {
	if n <= 0 || n > 1048576 {
		return nil, errors.New("invalid read size")
	}
	b := make([]byte, n)
	var read uintptr
	r, _, e := readMemory.Call(m.h, uintptr(a), uintptr(unsafe.Pointer(&b[0])), uintptr(n), uintptr(unsafe.Pointer(&read)))
	if r == 0 || int(read) != n {
		return nil, winerr("memory read failed", e)
	}
	return b, nil
}
func (m *winMemory) Write(a uint64, b []byte) error {
	if len(b) == 0 {
		return errors.New("empty write")
	}
	var old uint32
	r, _, e := protectMemory.Call(m.h, uintptr(a), uintptr(len(b)), 0x40, uintptr(unsafe.Pointer(&old)))
	if r == 0 {
		return winerr("protect failed", e)
	}
	var written uintptr
	r, _, e = writeMemory.Call(m.h, uintptr(a), uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), uintptr(unsafe.Pointer(&written)))
	var result error
	if r == 0 || int(written) != len(b) {
		result = winerr("memory write failed", e)
	}
	r, _, e = flushCache.Call(m.h, uintptr(a), uintptr(len(b)))
	if r == 0 {
		result = errors.Join(result, winerr("cache flush failed", e))
	}
	var ignored uint32
	r, _, e = protectMemory.Call(m.h, uintptr(a), uintptr(len(b)), uintptr(old), uintptr(unsafe.Pointer(&ignored)))
	if r == 0 {
		result = errors.Join(result, winerr("restoring page protection failed", e))
	}
	return result
}
func (m *winMemory) Pause() (func() error, error) {
	var threads []uintptr
	resume := func() error {
		var result error
		for i := len(threads) - 1; i >= 0; i-- {
			r, _, e := resumeThread.Call(threads[i])
			if uint32(r) == 0xffffffff {
				result = errors.Join(result, winerr("resume thread failed", e))
			}
			closeHandle.Call(threads[i])
		}
		threads = nil
		return result
	}
	snap, _, e := snapshot.Call(4, 0)
	if snap == ^uintptr(0) {
		return nil, e
	}
	defer closeHandle.Call(snap)
	te := threadEntry{}
	te.Size = uint32(unsafe.Sizeof(te))
	ok, _, _ := threadFirst.Call(snap, uintptr(unsafe.Pointer(&te)))
	for ok != 0 {
		if int(te.PID) == m.info.PID {
			h, _, e := openThread.Call(2, 0, uintptr(te.TID))
			if h == 0 {
				return nil, errors.Join(e, resume())
			}
			r, _, e := suspendThread.Call(h)
			if uint32(r) == 0xffffffff {
				closeHandle.Call(h)
				return nil, errors.Join(e, resume())
			}
			threads = append(threads, h)
		}
		ok, _, _ = threadNext.Call(snap, uintptr(unsafe.Pointer(&te)))
	}
	if len(threads) == 0 {
		return nil, errors.New("no live game threads")
	}
	return resume, nil
}
func (m *winMemory) Alive() bool { r, _, _ := waitProcess.Call(m.h, 0); return r == 258 }
func (m *winMemory) Close() error {
	if m.h != 0 {
		closeHandle.Call(m.h)
		m.h = 0
	}
	return nil
}
func PlatformKeys(pid int) (bool, int) {
	w, _, _ := foreground.Call()
	var focused uint32
	windowPID.Call(w, uintptr(unsafe.Pointer(&focused)))
	if int(focused) != pid {
		return false, 0
	}
	down := func(k uintptr) bool { v, _, _ := asyncKey.Call(k); return v&0x8000 != 0 }
	direction := 0
	if down('W') || down(0x26) || down(0x20) {
		direction++
	}
	if down('S') || down(0x28) {
		direction--
	}
	return down('C') && down('G') && down('D'), direction
}
func GlobalKeysAvailable() bool { return true }
