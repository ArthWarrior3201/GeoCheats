package core

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type linuxMemory struct {
	file *os.File
	info ProcessInfo
}

func FindProcesses(path string) ([]ProcessInfo, error) {
	target, _ := filepath.EvalSymlinks(path)
	var result []ProcessInfo
	dirs, e := os.ReadDir("/proc")
	if e != nil {
		return nil, e
	}
	for _, d := range dirs {
		pid, e := strconv.Atoi(d.Name())
		if e != nil || !d.IsDir() {
			continue
		}
		f, e := os.Open("/proc/" + d.Name() + "/maps")
		if e != nil {
			continue
		}
		scan := bufio.NewScanner(f)
		var base, end uint64
		for scan.Scan() {
			line := scan.Text()
			parts := strings.Fields(line)
			if len(parts) < 6 {
				continue
			}
			mapped := strings.Join(parts[5:], " ")
			mapped = strings.TrimSuffix(mapped, " (deleted)")
			if mapped != target && mapped != path {
				continue
			}
			bounds := strings.Split(parts[0], "-")
			if len(bounds) != 2 {
				continue
			}
			start, _ := strconv.ParseUint(bounds[0], 16, 64)
			stop, _ := strconv.ParseUint(bounds[1], 16, 64)
			offset, _ := strconv.ParseUint(parts[2], 16, 64)
			candidate := start - offset
			if base == 0 || candidate < base {
				base = candidate
			}
			if stop > end {
				end = stop
			}
		}
		f.Close()
		if base != 0 { // Header mapping may not account for all PE sections under Wine; use PE image size later.
			result = append(result, ProcessInfo{pid, path, base, end - base})
		}
	}
	return result, nil
}
func OpenMemory(info ProcessInfo) (Memory, error) {
	f, e := os.OpenFile(fmt.Sprintf("/proc/%d/mem", info.PID), os.O_RDWR, 0)
	if e != nil {
		return nil, fmt.Errorf("Linux denied access to the game memory: %w. This app does not change ptrace/security settings. The Windows GeoCheats build can alternatively run inside the game's Proton/Wine environment", e)
	}
	return &linuxMemory{f, info}, nil
}
func (m *linuxMemory) Info() ProcessInfo { return m.info }
func (m *linuxMemory) Read(a uint64, n int) ([]byte, error) {
	if n <= 0 || n > 1048576 {
		return nil, errors.New("invalid read size")
	}
	b := make([]byte, n)
	got, e := m.file.ReadAt(b, int64(a))
	if e != nil || got != n {
		return nil, fmt.Errorf("read game memory: %w", e)
	}
	return b, nil
}
func (m *linuxMemory) Write(a uint64, b []byte) error {
	n, e := m.file.WriteAt(b, int64(a))
	if e != nil {
		return e
	}
	if n != len(b) {
		return errors.New("short memory write")
	}
	return nil
}
func (m *linuxMemory) Alive() bool {
	_, e := os.Stat(fmt.Sprintf("/proc/%d", m.info.PID))
	return e == nil
}
func (m *linuxMemory) Close() error { return m.file.Close() }
func (m *linuxMemory) Pause() (func() error, error) {
	// Attach every existing thread, preserving independently stopped threads.
	// Linux PTRACE operations are OS-thread scoped; pin this goroutine until detach.
	lockThread()
	var tids []int
	resume := func() error {
		var result error
		for i := len(tids) - 1; i >= 0; i-- {
			if e := syscall.PtraceDetach(tids[i]); e != nil && e != syscall.ESRCH {
				result = errors.Join(result, e)
			}
		}
		tids = nil
		unlockThread()
		return result
	}
	entries, e := os.ReadDir(fmt.Sprintf("/proc/%d/task", m.info.PID))
	if e != nil {
		_ = resume()
		return nil, e
	}
	for _, entry := range entries {
		tid, e := strconv.Atoi(entry.Name())
		if e != nil {
			continue
		}
		if e = syscall.PtraceAttach(tid); e != nil {
			_ = resume()
			return nil, fmt.Errorf("ptrace attach denied: %w", e)
		}
		tids = append(tids, tid)
		var status syscall.WaitStatus
		_, e = syscall.Wait4(tid, &status, 0x40000000, nil)
		if e != nil {
			_ = resume()
			return nil, e
		}
		if !status.Stopped() {
			_ = resume()
			return nil, errors.New("game thread did not stop")
		}
	}
	if len(tids) == 0 {
		_ = resume()
		return nil, errors.New("no game threads")
	}
	return resume, nil
}
