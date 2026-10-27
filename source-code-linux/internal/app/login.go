package app

import (
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// User-visible login items only. Removal is supported; no driver or system service.
func configureLogin(enabled bool) error {
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	exe, e = filepath.Abs(exe)
	if e != nil {
		return e
	}
	home, e := os.UserHomeDir()
	if e != nil {
		return e
	}
	var path, content string
	switch runtime.GOOS {
	case "windows":
		key := `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`
		if !enabled {
			if e := exec.Command("reg.exe", "query", key, "/v", "GeoCheats").Run(); e != nil {
				return nil
			}
			return exec.Command("reg.exe", "delete", key, "/v", "GeoCheats", "/f").Run()
		}
		return exec.Command("reg.exe", "add", key, "/v", "GeoCheats", "/t", "REG_SZ", "/d", "\""+exe+"\" --background", "/f").Run()
	case "darwin":
		path = filepath.Join(home, "Library", "LaunchAgents", "io.geocheats.controller.plist")
		var b strings.Builder
		_ = xml.EscapeText(&b, []byte(exe))
		content = `<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict><key>Label</key><string>io.geocheats.controller</string><key>ProgramArguments</key><array><string>` + b.String() + `</string><string>--background</string></array><key>RunAtLoad</key><true/></dict></plist>`
	case "linux":
		config, e := os.UserConfigDir()
		if e != nil {
			return e
		}
		path = filepath.Join(config, "autostart", "geocheats.desktop")
		// Desktop Entry Exec escaping, including field-code percent escaping.
		escaped := strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "`", "\\`", "$", "\\$", "%", "%%").Replace(exe)
		escaped = strings.ReplaceAll(escaped, "\\", "\\\\")
		if strings.ContainsAny(exe, "\r\n") {
			return fmt.Errorf("unsupported newline in executable path")
		}
		content = "[Desktop Entry]\nType=Application\nName=GeoCheats\nComment=Restore your remembered Geometry Dash mods\nExec=\"" + escaped + "\" --background\nTerminal=false\nX-GNOME-Autostart-enabled=true\n"
	default:
		return fmt.Errorf("login start unsupported on %s", runtime.GOOS)
	}
	if !enabled {
		e := os.Remove(path)
		if os.IsNotExist(e) {
			return nil
		}
		return e
	}
	return atomicFile(path, []byte(content))
}
