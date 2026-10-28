package core

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/ebitengine/purego"
	"os"
	"runtime"
	"strings"
	"sync"
	"unsafe"
)

var macOnce sync.Once
var macErr error
var taskSelf uint32
var listPIDs func(uint32, uint32, unsafe.Pointer, int32) int32
var pidPath func(int32, unsafe.Pointer, uint32) int32
var getTask func(uint32, int32, *uint32) int32
var readVM func(uint32, uint64, uint64, unsafe.Pointer, *uint64) int32
var writeVM func(uint32, uint64, unsafe.Pointer, uint32) int32
var protectVM func(uint32, uint64, uint64, int32, int32) int32
var regionVM func(uint32, *uint64, *uint64, int32, unsafe.Pointer, *uint32, *uint32) int32
var suspendTask func(uint32) int32
var resumeTask func(uint32) int32
var deallocatePort func(uint32, uint32) int32
var taskInfo func(uint32, int32, unsafe.Pointer, *uint32) int32

func initMac() error {
	macOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				macErr = fmt.Errorf("macOS API binding: %v", r)
			}
		}()
		lib, e := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if e != nil {
			macErr = e
			return
		}
		procLib, e := purego.Dlopen("/usr/lib/libproc.dylib", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if e != nil {
			macErr = e
			return
		}
		port, e := purego.Dlsym(lib, "mach_task_self_")
		if e != nil {
			macErr = e
			return
		}
		taskSelf = *(*uint32)(unsafe.Pointer(port))
		purego.RegisterLibFunc(&listPIDs, procLib, "proc_listpids")
		purego.RegisterLibFunc(&pidPath, procLib, "proc_pidpath")
		purego.RegisterLibFunc(&getTask, lib, "task_for_pid")
		purego.RegisterLibFunc(&readVM, lib, "mach_vm_read_overwrite")
		purego.RegisterLibFunc(&writeVM, lib, "mach_vm_write")
		purego.RegisterLibFunc(&protectVM, lib, "mach_vm_protect")
		purego.RegisterLibFunc(&regionVM, lib, "mach_vm_region")
		purego.RegisterLibFunc(&taskInfo, lib, "task_info")
		purego.RegisterLibFunc(&suspendTask, lib, "task_suspend")
		purego.RegisterLibFunc(&resumeTask, lib, "task_resume")
		purego.RegisterLibFunc(&deallocatePort, lib, "mach_port_deallocate")
	})
	return macErr
}
func processPath(pid int) string {
	b := make([]byte, 4096)
	n := pidPath(int32(pid), unsafe.Pointer(&b[0]), uint32(len(b)))
	runtime.KeepAlive(b)
	if n <= 0 || int(n) > len(b) {
		return ""
	}
	return strings.TrimRight(string(b[:n]), "\x00")
}
func FindProcesses(path string) ([]ProcessInfo, error) {
	if e := initMac(); e != nil {
		return nil, e
	}
	size := listPIDs(1, 0, nil, 0)
	if size <= 0 {
		return nil, errors.New("proc_listpids failed")
	}
	pids := make([]int32, size/4+1024)
	n := listPIDs(1, 0, unsafe.Pointer(&pids[0]), int32(len(pids)*4))
	runtime.KeepAlive(pids)
	if n < 0 || int(n)/4 > len(pids) {
		return nil, errors.New("proc_listpids returned an invalid process count")
	}
	var result []ProcessInfo
	for _, pid := range pids[:int(n)/4] {
		if pid > 0 && processPath(int(pid)) == path {
			result = append(result, ProcessInfo{PID: int(pid), Path: path})
		}
	}
	return result, nil
}

type macMemory struct {
	task uint32
	info ProcessInfo
}

func OpenMemory(info ProcessInfo) (Memory, error) {
	if e := initMac(); e != nil {
		return nil, e
	}
	var task uint32
	if code := getTask(taskSelf, int32(info.PID), &task); code != 0 {
		return nil, fmt.Errorf("macOS denied task_for_pid (Mach %d). This app does not disable SIP or alter game signatures/entitlements", code)
	}
	m := &macMemory{task: task, info: info}
	// TASK_DYLD_INFO gives the main image load address. PE offsets are never reused for a Mach-O game.
	var dyld [3]uint64
	count := uint32(5)
	if code := taskInfo(task, 17, unsafe.Pointer(&dyld[0]), &count); code != 0 {
		_ = m.Close()
		return nil, fmt.Errorf("task_info Mach %d", code)
	}
	header, e := m.Read(dyld[0], 16)
	if e != nil {
		_ = m.Close()
		return nil, e
	}
	array := binary.LittleEndian.Uint64(header[8:])
	entry, e := m.Read(array, 24)
	if e != nil {
		_ = m.Close()
		return nil, e
	}
	m.info.Base = binary.LittleEndian.Uint64(entry)
	im, e := Inspect(info.Path)
	if e != nil {
		_ = m.Close()
		return nil, e
	}
	m.info.Size = uint64(im.Size) + 64*1024*1024
	// Runtime magic must match a 64-bit Mach-O image.
	magic, e := m.Read(m.info.Base, 4)
	if e != nil || binary.LittleEndian.Uint32(magic) != 0xfeedfacf {
		_ = m.Close()
		return nil, errors.New("dyld main image is not a thin 64-bit Mach-O image")
	}
	return m, nil
}
func (m *macMemory) Info() ProcessInfo { return m.info }
func (m *macMemory) Read(a uint64, n int) ([]byte, error) {
	if n <= 0 || n > 1048576 {
		return nil, errors.New("invalid read size")
	}
	b := make([]byte, n)
	var got uint64
	code := readVM(m.task, a, uint64(n), unsafe.Pointer(&b[0]), &got)
	runtime.KeepAlive(b)
	if code != 0 || got != uint64(n) {
		return nil, fmt.Errorf("mach_vm_read failed (%d)", code)
	}
	return b, nil
}
func (m *macMemory) Write(a uint64, b []byte) (err error) {
	if len(b) == 0 {
		return errors.New("empty write")
	}
	address := a
	var size uint64
	info := [16]int32{}
	count := uint32(9)
	var object uint32
	if code := regionVM(m.task, &address, &size, 9, unsafe.Pointer(&info[0]), &count, &object); code != 0 {
		return fmt.Errorf("mach_vm_region failed (%d)", code)
	}
	if object != 0 {
		_ = deallocatePort(taskSelf, object)
	}
	if a < address || a+uint64(len(b)) > address+size {
		return errors.New("write crosses a Mach region")
	}
	page := uint64(os.Getpagesize())
	start := a &^ (page - 1)
	length := (a + uint64(len(b)) + page - 1) &^ (page - 1)
	length -= start
	old := info[0]
	if code := protectVM(m.task, start, length, 0, old|2|0x10); code != 0 {
		return fmt.Errorf("macOS denied writable code protection (%d)", code)
	}
	defer func() {
		if code := protectVM(m.task, start, length, 0, old); code != 0 {
			err = errors.Join(err, fmt.Errorf("restore protection failed (%d)", code))
		}
	}()
	code := writeVM(m.task, a, unsafe.Pointer(&b[0]), uint32(len(b)))
	runtime.KeepAlive(b)
	if code != 0 {
		return fmt.Errorf("mach_vm_write failed (%d)", code)
	}
	// ARM64 instruction-cache synchronization requires target-side support. Until implemented, native ARM code writes are rejected before reaching here by the session.
	return nil
}
func (m *macMemory) Pause() (func() error, error) {
	if code := suspendTask(m.task); code != 0 {
		return nil, fmt.Errorf("task_suspend failed (%d)", code)
	}
	return func() error {
		if code := resumeTask(m.task); code != 0 {
			return fmt.Errorf("task_resume failed (%d)", code)
		}
		return nil
	}, nil
}
func (m *macMemory) Alive() bool { return m.task != 0 && processPath(m.info.PID) == m.info.Path }
func (m *macMemory) Close() error {
	if m.task != 0 {
		_ = deallocatePort(taskSelf, m.task)
		m.task = 0
	}
	return nil
}
func PlatformKeys(pid int) (bool, int) { return false, 0 }
func GlobalKeysAvailable() bool        { return false }
