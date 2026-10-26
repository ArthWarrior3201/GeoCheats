package core

import "runtime"

func lockThread()   { runtime.LockOSThread() }
func unlockThread() { runtime.UnlockOSThread() }
