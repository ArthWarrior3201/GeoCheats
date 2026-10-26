package core

import (
	"github.com/ebitengine/purego"
	"os"
	"sync"
	"unsafe"
)

var xOnce sync.Once
var xDisplay uintptr
var xQuery func(uintptr, *byte) int32
var xCode func(uintptr, uintptr) uint8
var xRoot func(uintptr) uintptr
var xAtom func(uintptr, string, int32) uintptr
var xProperty func(uintptr, uintptr, uintptr, int64, int64, int32, uintptr, *uintptr, *int32, *uint64, *uint64, *uintptr) int32
var xFree func(uintptr) int32

func initX() {
	xOnce.Do(func() {
		if os.Getenv("DISPLAY") == "" || os.Getenv("XDG_SESSION_TYPE") == "wayland" {
			return
		}
		defer func() {
			if recover() != nil {
				xDisplay = 0
			}
		}()
		lib, e := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if e != nil {
			return
		}
		var open func(uintptr) uintptr
		purego.RegisterLibFunc(&open, lib, "XOpenDisplay")
		purego.RegisterLibFunc(&xQuery, lib, "XQueryKeymap")
		purego.RegisterLibFunc(&xCode, lib, "XKeysymToKeycode")
		purego.RegisterLibFunc(&xRoot, lib, "XDefaultRootWindow")
		purego.RegisterLibFunc(&xAtom, lib, "XInternAtom")
		purego.RegisterLibFunc(&xProperty, lib, "XGetWindowProperty")
		purego.RegisterLibFunc(&xFree, lib, "XFree")
		xDisplay = open(0)
	})
}
func GlobalKeysAvailable() bool { initX(); return xDisplay != 0 }
func xValue(window uintptr, property string) uint64 {
	var actual, data uintptr
	var format int32
	var n, after uint64
	atom := xAtom(xDisplay, property, 1)
	if atom == 0 {
		return 0
	}
	r := xProperty(xDisplay, window, atom, 0, 1, 0, 0, &actual, &format, &n, &after, &data)
	if data != 0 {
		defer xFree(data)
	}
	if r != 0 || data == 0 || n == 0 || format != 32 {
		return 0
	}
	return *(*uint64)(unsafe.Pointer(data))
}
func PlatformKeys(pid int) (bool, int) {
	if !GlobalKeysAvailable() {
		return false, 0
	}
	window := xValue(xRoot(xDisplay), "_NET_ACTIVE_WINDOW")
	if window == 0 || int(xValue(uintptr(window), "_NET_WM_PID")) != pid {
		return false, 0
	}
	var bits [32]byte
	xQuery(xDisplay, &bits[0])
	down := func(sym uintptr) bool { key := xCode(xDisplay, sym); return bits[key/8]&(1<<(key%8)) != 0 }
	direction := 0
	if down('w') || down(' ') || down(0xff52) {
		direction++
	}
	if down('s') || down(0xff54) {
		direction--
	}
	return down('c') && down('g') && down('d'), direction
}
