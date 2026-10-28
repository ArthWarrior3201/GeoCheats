package app

import (
	"fmt"
	"geocheats/internal/cocoaui"
	"os"
)

func Run() {
	root := dataRoot()
	check := false
	for _, arg := range os.Args {
		if arg == "--launch-check" {
			check = true
		}
	}
	if check {
		var e error
		root, e = os.MkdirTemp("", "geocheats-appkit-check-")
		if e != nil {
			fmt.Println(e)
			return
		}
		defer os.RemoveAll(root)
	}
	service, e := newService(root, true)
	if e != nil {
		if e != errAlreadyRunning {
			fmt.Println(e)
		}
		return
	}
	defer service.Close()
	service.Start()
	state := func() map[string]any {
		v := service.Snapshot()
		mods := make([]map[string]any, 0, len(v.Mods))
		for _, m := range v.Mods {
			mods = append(mods, map[string]any{"id": m.Package.ID, "name": m.Package.Name, "enabled": m.Enabled, "active": m.Active, "compatible": m.Compatible})
		}
		show := false
		select {
		case <-service.wake:
			show = true
		default:
		}
		return map[string]any{"linked": v.Linked, "mods": mods, "message": v.Message, "login": v.LoginStart, "show": show}
	}
	cocoaui.Run("GeoCheats", func(action string, input map[string]any) map[string]any {
		text := func(key string) string { s, _ := input[key].(string); return s }
		on, _ := input["enabled"].(bool)
		var e error
		switch action {
		case "state":
			return state()
		case "link":
			e = service.Link(text("path"))
		case "toggle":
			e = service.Toggle(text("id"), on)
		case "login":
			e = service.Login(on)
		case "launch":
			e = service.Launch()
		case "trigger":
			e = service.Trigger()
		case "import":
			var b []byte
			b, e = os.ReadFile(text("path"))
			if e == nil {
				e = service.Import(b)
			}
		case "quit":
			e = service.action("disconnect", request{})
			if e == nil {
				e = service.Close()
			}
		}
		if e != nil {
			return map[string]any{"error": e.Error()}
		}
		return state()
	})
}
