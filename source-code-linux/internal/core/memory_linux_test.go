package core

import (
 "os"
 "runtime"
 "testing"
 "unsafe"
)
func TestLinuxOwnMemoryRoundTrip(t *testing.T){
 data:=[8]byte{1,2,3,4,5,6,7,8};address:=uint64(uintptr(unsafe.Pointer(&data[0])))
 memory,e:=OpenMemory(ProcessInfo{PID:os.Getpid()});if e!=nil{t.Skipf("OS denies test access: %v",e)};defer memory.Close()
 b,e:=memory.Read(address,8);if e!=nil{t.Fatal(e)};if !Equal(b,data[:]){t.Fatal("read mismatch")}
 changed:=[]byte{8,7,6,5,4,3,2,1};if e=memory.Write(address,changed);e!=nil{t.Fatal(e)};if !Equal(data[:],changed){t.Fatal("write mismatch")}
 if e=memory.Write(address,b);e!=nil{t.Fatal(e)};if data[0]!=1{t.Fatal("restore mismatch")};runtime.KeepAlive(&data)
}
