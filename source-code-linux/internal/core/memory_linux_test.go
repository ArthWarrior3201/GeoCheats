package core

import (
 "os"
 "runtime"
 "testing"
 "unsafe"
)
var linuxMemoryProbe [8]byte
func TestLinuxOwnMemoryRoundTrip(t *testing.T){
 linuxMemoryProbe=[8]byte{1,2,3,4,5,6,7,8};data:=linuxMemoryProbe[:];address:=uint64(uintptr(unsafe.Pointer(&linuxMemoryProbe[0])))
 memory,e:=OpenMemory(ProcessInfo{PID:os.Getpid()});if e!=nil{t.Skipf("OS denies test access: %v",e)};defer memory.Close()
 b,e:=memory.Read(address,8);if e!=nil{t.Fatal(e)};if !Equal(b,data[:]){t.Fatal("read mismatch")}
 changed:=[]byte{8,7,6,5,4,3,2,1};if e=memory.Write(address,changed);e!=nil{t.Fatal(e)};observed,e:=memory.Read(address,8);if e!=nil||!Equal(observed,changed){t.Fatal("write mismatch",e)}
 if e=memory.Write(address,b);e!=nil{t.Fatal(e)};observed,e=memory.Read(address,8);if e!=nil||!Equal(observed,b){t.Fatal("restore mismatch",e)};runtime.KeepAlive(&linuxMemoryProbe)
}
