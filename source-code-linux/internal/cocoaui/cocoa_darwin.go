package cocoaui

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
void gc_native_run(const char *name,const char *state,const char *report);
*/
import "C"

import (
	"encoding/json"
	"os"
	"runtime"
	"unsafe"
)

var handler func(string, map[string]any) map[string]any

func init() { runtime.LockOSThread() }

//export gc_go_event
func gc_go_event(event, input *C.char) *C.char {
	values := map[string]any{}
	_ = json.Unmarshal([]byte(C.GoString(input)), &values)
	result := handler(C.GoString(event), values)
	b, e := json.Marshal(result)
	if e != nil {
		b = []byte(`{"error":"Could not update controls"}`)
	}
	return C.CString(string(b))
}

func Run(name string, callback func(string, map[string]any) map[string]any) {
	runtime.LockOSThread()
	handler = callback
	b, _ := json.Marshal(callback("state", nil))
	report := ""
	for i, arg := range os.Args {
		if arg == "--launch-check" && i+1 < len(os.Args) {
			report = os.Args[i+1]
		}
	}
	n, s, r := C.CString(name), C.CString(string(b)), C.CString(report)
	defer C.free(unsafe.Pointer(n))
	defer C.free(unsafe.Pointer(s))
	defer C.free(unsafe.Pointer(r))
	C.gc_native_run(n, s, r)
}
