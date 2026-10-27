package core

import (
	"os"
	"runtime"
	"testing"
	"unsafe"
)

var macMemoryProbe = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}

func TestMacOwnMemoryRoundTrip(t *testing.T) {
	if e := initMac(); e != nil {
		t.Fatal(e)
	}
	path := processPath(os.Getpid())
	if path == "" {
		t.Fatal("proc_pidpath failed for own process")
	}
	processes, e := FindProcesses(path)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, p := range processes {
		found = found || p.PID == os.Getpid()
	}
	if !found {
		t.Fatal("own process missing from enumeration")
	}
	mem, e := OpenMemory(ProcessInfo{PID: os.Getpid(), Path: path})
	if e != nil {
		t.Fatal(e)
	}
	defer mem.Close()
	address := uint64(uintptr(unsafe.Pointer(&macMemoryProbe[0])))
	before, e := mem.Read(address, 8)
	if e != nil {
		t.Fatal(e)
	}
	changed := []byte{8, 7, 6, 5, 4, 3, 2, 1}
	if e = mem.Write(address, changed); e != nil {
		t.Fatal(e)
	}
	defer func() {
		if e := mem.Write(address, before); e != nil {
			t.Error(e)
		}
	}()
	after, e := mem.Read(address, 8)
	if e != nil || !Equal(after, changed) {
		t.Fatalf("write readback: %v %v", after, e)
	}
	if e = mem.Write(address, before); e != nil {
		t.Fatal(e)
	}
	restored, e := mem.Read(address, 8)
	if e != nil || !Equal(restored, before) {
		t.Fatal("restoration mismatch")
	}
	runtime.KeepAlive(&macMemoryProbe)
}
