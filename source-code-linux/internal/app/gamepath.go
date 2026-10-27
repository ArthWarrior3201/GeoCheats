package app

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// ResolveExecutable accepts a Windows/Proton executable or a native Mac .app bundle.
func ResolveExecutable(path string) (string, error) { return resolve(path) }
func InstalledGames() []string                      { return discover() }

func resolve(path string) (string, error) {
	path = strings.Trim(strings.TrimSpace(path), "\"")
	if path == "" {
		return "", errors.New("choose Geometry Dash")
	}
	if strings.EqualFold(filepath.Ext(path), ".app") {
		dir := filepath.Join(path, "Contents", "MacOS")
		// CFBundleExecutable is authoritative; plutil handles both XML and binary plists.
		if runtime.GOOS == "darwin" {
			if b, e := exec.Command("/usr/bin/plutil", "-extract", "CFBundleExecutable", "raw", "-o", "-", filepath.Join(path, "Contents", "Info.plist")).Output(); e == nil {
				name := strings.TrimSpace(string(b))
				if name != "" && filepath.Base(name) == name {
					p := filepath.Join(dir, name)
					if st, e := os.Stat(p); e == nil && st.Mode().IsRegular() {
						return filepath.Abs(p)
					}
				}
			}
		}
		entries, e := os.ReadDir(dir)
		if e != nil {
			return "", e
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.Contains(strings.ToLower(entry.Name()), "geometry") {
				return filepath.Abs(filepath.Join(dir, entry.Name()))
			}
		}
		return "", errors.New("this app bundle has no Geometry Dash executable")
	}
	return filepath.Abs(path)
}

func discover() []string {
	home, _ := os.UserHomeDir()
	var roots []string
	switch runtime.GOOS {
	case "windows":
		for _, name := range []string{"ProgramFiles(x86)", "ProgramFiles"} {
			if root := os.Getenv(name); root != "" {
				roots = append(roots, filepath.Join(root, "Steam"))
			}
		}
		for _, drive := range []string{"C:", "D:", "E:", "F:"} {
			roots = append(roots, drive+`\SteamLibrary`)
		}
	case "darwin":
		roots = []string{filepath.Join(home, "Library", "Application Support", "Steam")}
	case "linux":
		roots = []string{filepath.Join(home, ".steam", "steam"), filepath.Join(home, ".steam", "root"), filepath.Join(home, ".local", "share", "Steam"), filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam"), filepath.Join(home, "snap", "steam", "common", ".local", "share", "Steam")}
	}
	found := discoverRoots(roots)
	if runtime.GOOS == "darwin" {
		for _, root := range []string{"/Applications", filepath.Join(home, "Applications")} {
			for _, name := range []string{"GeometryDash.app", "Geometry Dash.app"} {
				path := filepath.Join(root, name)
				if _, e := os.Stat(path); e == nil {
					found = append(found, path)
				}
			}
		}
	}
	return found
}

func discoverRoots(roots []string) []string {
	field := func(b []byte, name string) []string {
		re := regexp.MustCompile(`"` + regexp.QuoteMeta(name) + `"\s+"([^"\r\n]+)"`)
		var out []string
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			out = append(out, strings.ReplaceAll(m[1], `\\`, `\`))
		}
		return out
	}
	for _, root := range append([]string(nil), roots...) {
		if b, e := os.ReadFile(filepath.Join(root, "steamapps", "libraryfolders.vdf")); e == nil {
			roots = append(roots, field(b, "path")...)
		}
	}
	seen := map[string]bool{}
	var found []string
	for _, root := range roots {
		dirs := []string{"Geometry Dash", "GeometryDash"}
		if b, e := os.ReadFile(filepath.Join(root, "steamapps", "appmanifest_322170.acf")); e == nil {
			dirs = append(field(b, "installdir"), dirs...)
		}
		for _, dir := range dirs {
			if dir == "" || filepath.IsAbs(dir) || strings.ContainsAny(dir, "/\\") {
				continue
			}
			base := filepath.Join(root, "steamapps", "common", dir)
			names := []string{"GeometryDash.exe", "GeometryDash.app", "Geometry Dash.app"}
			if runtime.GOOS == "darwin" {
				names = []string{"GeometryDash.app", "Geometry Dash.app", "GeometryDash.exe"}
			}
			if entries, e := os.ReadDir(base); e == nil {
				for _, entry := range entries {
					n := entry.Name()
					if strings.Contains(strings.ToLower(n), "geometry") && (strings.EqualFold(filepath.Ext(n), ".app") || strings.EqualFold(filepath.Ext(n), ".exe")) {
						names = append(names, n)
					}
				}
			}
			for _, name := range names {
				p := filepath.Join(base, name)
				if _, e := os.Stat(p); e == nil && !seen[p] {
					seen[p] = true
					found = append(found, p)
				}
			}
		}
	}
	return found
}
