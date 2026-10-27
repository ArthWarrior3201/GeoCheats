package app

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"geocheats/internal/core"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

//go:embed ui.html
var assets embed.FS

type controller struct {
	mu                  sync.Mutex
	path, root, message string
	image               *core.Image
	session             *core.Session
	mods                map[string]core.Package
	triggerUntil        time.Time
	chord               bool
	enabled             map[string]string
	loginStart          bool
	loginConfigured     bool
	suspended           bool
	nextConnect         time.Time
}
type request struct {
	Path      string          `json:"path"`
	ID        string          `json:"id"`
	Enabled   bool            `json:"enabled"`
	Direction int             `json:"direction"`
	Package   json.RawMessage `json:"package"`
}

func dataRoot() string {
	d, e := os.UserConfigDir()
	if e != nil {
		d = os.TempDir()
	}
	return filepath.Join(d, "GeoCheats")
}
func resolve(path string) (string, error) {
	if path == "" {
		return "", errors.New("choose or paste the installed game's executable path")
	}
	path = strings.Trim(strings.TrimSpace(path), "\"")
	if strings.HasSuffix(strings.ToLower(path), ".app") {
		dir := filepath.Join(path, "Contents", "MacOS")
		entries, e := os.ReadDir(dir)
		if e != nil {
			return "", e
		}
		for _, f := range entries {
			if !f.IsDir() && strings.Contains(strings.ToLower(f.Name()), "geometry") {
				path = filepath.Join(dir, f.Name())
				break
			}
		}
	}
	path, e := filepath.Abs(path)
	if e != nil {
		return "", e
	}
	return path, nil
}
func discover() []string {
	home, _ := os.UserHomeDir()
	roots := []string{}
	switch runtime.GOOS {
	case "windows":
		for _, e := range []string{"ProgramFiles(x86)", "ProgramFiles"} {
			if p := os.Getenv(e); p != "" {
				roots = append(roots, filepath.Join(p, "Steam"))
			}
		}
		for _, d := range []string{"C:", "D:", "E:", "F:"} {
			roots = append(roots, d+`\SteamLibrary`)
		}
	case "linux":
		roots = []string{filepath.Join(home, ".steam", "steam"), filepath.Join(home, ".steam", "root"), filepath.Join(home, ".local", "share", "Steam"), filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam")}
	case "darwin":
		roots = []string{filepath.Join(home, "Library", "Application Support", "Steam")}
	}
	pattern := regexp.MustCompile(`"path"\s+"([^"\r\n]+)"`)
	for _, root := range append([]string{}, roots...) {
		b, e := os.ReadFile(filepath.Join(root, "steamapps", "libraryfolders.vdf"))
		if e != nil {
			continue
		}
		for _, m := range pattern.FindAllStringSubmatch(string(b), -1) {
			roots = append(roots, strings.ReplaceAll(m[1], `\\`, `\`))
		}
	}
	seen := map[string]bool{}
	var result []string
	for _, root := range roots {
		base := filepath.Join(root, "steamapps", "common", "Geometry Dash")
		for _, name := range []string{"GeometryDash.exe", "Geometry Dash.app"} {
			p := filepath.Join(base, name)
			if _, e := os.Stat(p); e == nil && !seen[p] {
				result = append(result, p)
				seen[p] = true
			}
		}
	}
	return result
}
func (c *controller) load() {
	_ = os.MkdirAll(filepath.Join(c.root, "mods"), 0700)
	c.mods = core.Builtins()
	files, _ := filepath.Glob(filepath.Join(c.root, "mods", "*.dash"))
	for _, f := range files {
		b, e := os.ReadFile(f)
		if e == nil {
			p, e := core.Decode(b)
			if e == nil {
				c.mods[p.ID] = p
			}
		}
	}
	if b, e := os.ReadFile(filepath.Join(c.root, "linked-game.txt")); e == nil {
		c.path = string(b)
		if im, e := core.Inspect(c.path); e == nil {
			c.image = &im
		}
	}
	c.loadSettings()
	c.prepareMacMod()
}
func (c *controller) state() map[string]any {
	mods := []map[string]any{}
	for _, p := range c.mods {
		on := false
		if c.session != nil {
			_, on = c.session.Active[p.ID]
		}
		mods = append(mods, map[string]any{"package": p, "on": on, "remembered": c.enabled[p.ID] == p.Target, "compatible": c.image != nil && c.image.SHA256 == p.Target})
	}
	connected := c.session != nil && c.session.Mem.Alive()
	pid := 0
	if connected {
		pid = c.session.Mem.Info().PID
	}
	return map[string]any{"name": "GeoCheats", "version": "1.1.0-experimental", "platform": runtime.GOOS + "/" + runtime.GOARCH, "image": c.image, "path": c.path, "connected": connected, "pid": pid, "mods": mods, "message": c.message, "detected": discover(), "globalKeys": core.GlobalKeysAvailable(), "loginStart": c.loginStart}
}
func (c *controller) action(action string, r request) error {
	switch action {
	case "link":
		p, e := resolve(r.Path)
		if e != nil {
			return e
		}
		im, e := core.Inspect(p)
		if e != nil {
			return e
		}
		if !strings.Contains(strings.ToLower(filepath.Base(p)), "geometry") {
			return errors.New("select the Geometry Dash executable")
		}
		if c.session != nil {
			if e = c.session.Close(); e != nil {
				return e
			}
			c.session = nil
		}
		if e = os.WriteFile(filepath.Join(c.root, "linked-game.txt"), []byte(p), 0600); e != nil {
			return e
		}
		c.path = p
		c.image = &im
		c.prepareMacMod()
		c.message = "Linked while closed or running. Connect is a separate step."
	case "launch":
		if c.image == nil {
			return errors.New("link your installed game first")
		}
		return openURL("steam://rungameid/322170")
	case "connect":
		c.suspended = false
		if c.image == nil {
			return errors.New("link the game first")
		}
		im, e := core.Inspect(c.path)
		if e != nil {
			return e
		}
		if im.SHA256 != c.image.SHA256 {
			return errors.New("game changed since linking; link it again")
		}
		if runtime.GOOS != "darwin" && (im.Format != "pe" || im.Arch != "amd64") {
			return errors.New("this backend supports the AMD64 Windows game, including Proton on Linux")
		}
		if runtime.GOOS == "darwin" && (im.Format != "macho" || im.Arch != "amd64") {
			return errors.New("native macOS byte-patch backend currently requires a thin Intel Mach-O game and matching profile; the bundled mods target Windows/Proton. ARM64/universal game slices need separate analysis")
		}
		if c.session != nil {
			if e = c.session.Close(); e != nil {
				return e
			}
			c.session = nil
		}
		list, e := core.FindProcesses(c.path)
		if e != nil {
			return e
		}
		if len(list) != 1 {
			return fmt.Errorf("found %d matching running games; start exactly one, then Connect", len(list))
		}
		mem, e := core.OpenMemory(list[0])
		if e != nil {
			return e
		}
		c.session = core.NewSession(mem, im)
		if e := c.sessionJournal(); e != nil {
			_ = mem.Close()
			c.session = nil
			return e
		}
		c.message = "Connected. Restoring remembered switches."
		c.restoreEnabled()
	case "disconnect":
		if c.session != nil {
			if e := c.session.Close(); e != nil {
				return e
			}
			c.session = nil
		}
		c.suspended = true
		c.message = "Disconnected and runtime patches restored. Saved switches retained; Connect resumes them."
	case "toggle":
		return c.setEnabled(r.ID, r.Enabled)
	case "login-start":
		if e := configureLogin(r.Enabled); e != nil {
			return e
		}
		c.loginStart = r.Enabled
		c.loginConfigured = true
		return c.saveSettings()
	case "import":
		p, e := core.Decode(r.Package)
		if e != nil {
			return e
		}
		if c.enabled[p.ID] != "" {
			return errors.New("turn this remembered mod off before replacing it")
		}
		if c.session != nil {
			if _, ok := c.session.Active[p.ID]; ok {
				return errors.New("turn this mod off before replacing it")
			}
		}
		data, _ := json.MarshalIndent(p, "", "  ")
		safe := regexp.MustCompile(`[^a-zA-Z0-9_.-]`).ReplaceAllString(p.ID, "_")
		if safe == "." || safe == ".." {
			return errors.New("invalid mod ID")
		}
		if e = os.WriteFile(filepath.Join(c.root, "mods", safe+".dash"), data, 0600); e != nil {
			return e
		}
		c.mods[p.ID] = p
		c.message = "Imported " + p.Name
	case "trigger":
		if c.session == nil {
			return errors.New("connect first")
		}
		if e := c.session.Trigger(); e != nil {
			return e
		}
		c.triggerUntil = time.Now().Add(500 * time.Millisecond)
		c.message = "Requested the game's normal ending sequence."
	case "flight":
		if c.session != nil {
			c.session.SetFlight(r.Direction)
		}
	default:
		return errors.New("unknown action")
	}
	return nil
}
func openURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if e := cmd.Start(); e != nil {
		return e
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
func Run() {
	c := &controller{root: dataRoot(), message: "Link the installed game first. No Geode or separate runtime needed."}
	background := len(os.Args) > 1 && os.Args[1] == "--background"
	unlock, lockErr := instanceLock(c.root)
	if errors.Is(lockErr, errAlreadyRunning) {
		for attempt := 0; attempt < 10; attempt++ {
			if reuseRunning(c.root, background) {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		fmt.Println("GeoCheats is already running; its control window is starting or busy.")
		return
	}
	if lockErr != nil {
		fmt.Println(lockErr)
		return
	}
	defer unlock()
	c.load()
	if reuseRunning(c.root, background) {
		return
	}
	random := make([]byte, 24)
	if _, e := rand.Read(random); e != nil {
		fmt.Println(e)
		return
	}
	token := hex.EncodeToString(random)
	listener, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		fmt.Println(e)
		return
	}
	address := "http://" + listener.Addr().String()
	url := address + "/" + token + "/"
	instancePath := filepath.Join(c.root, "running.json")
	if e := atomicFile(instancePath, []byte(url)); e != nil {
		fmt.Println(e)
		_ = listener.Close()
		return
	}
	defer os.Remove(instancePath)
	mux := http.NewServeMux()
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Host != listener.Addr().String() {
			http.Error(w, "invalid host", 403)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; connect-src 'self'; frame-ancestors 'none'")
		if r.URL.Path == "/"+token+"/" && r.Method == "GET" {
			raw, _ := assets.ReadFile("ui.html")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(w, strings.ReplaceAll(string(raw), "__TOKEN__", token))
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/") || r.Header.Get("X-GeoCheats-Token") != token {
			http.Error(w, "not found", 404)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && origin != address {
			http.Error(w, "cross-origin request denied", 403)
			return
		}
		action := strings.TrimPrefix(r.URL.Path, "/api/")
		c.mu.Lock()
		defer c.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if action == "state" && r.Method == "GET" {
			_ = json.NewEncoder(w).Encode(c.state())
			return
		}
		if r.Method != "POST" {
			http.Error(w, "POST required", 405)
			return
		}
		var req request
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256*1024)).Decode(&req); e != nil {
			w.WriteHeader(400)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": e.Error()})
			return
		}
		if action == "quit" {
			if c.session != nil {
				if e := c.session.Close(); e != nil {
					w.WriteHeader(409)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": e.Error()})
					return
				}
				c.session = nil
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "Controller stopped; runtime patches restored. Saved switches remain for next launch/login."})
			go func() { time.Sleep(100 * time.Millisecond); _ = server.Shutdown(context.Background()) }()
			return
		}
		if e := c.action(action, req); e != nil {
			c.message = e.Error()
			w.WriteHeader(409)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": e.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(c.state())
	})
	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(8 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				c.mu.Lock()
				c.maintain(time.Now())
				if s := c.session; s != nil {
					chord, flight := core.PlatformKeys(s.Mem.Info().PID)
					if core.GlobalKeysAvailable() {
						s.SetFlight(flight)
					}
					if chord && !c.chord {
						if e := s.Trigger(); e != nil {
							c.message = e.Error()
						} else {
							c.triggerUntil = time.Now().Add(500 * time.Millisecond)
							c.message = "Completion hotkey sent to game."
						}
					}
					c.chord = chord
					if !c.triggerUntil.IsZero() && time.Now().After(c.triggerUntil) {
						if e := s.ClearTriggers(); e != nil {
							c.message = e.Error()
						} else {
							c.triggerUntil = time.Time{}
						}
					}
					if e := s.Tick(); e != nil {
						c.message = e.Error()
					}
				}
				c.mu.Unlock()
			}
		}
	}()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	go func() {
		select {
		case <-done:
			return
		case <-signals:
			c.mu.Lock()
			if c.session != nil {
				if e := c.session.Close(); e != nil {
					fmt.Println("Restore failed; restart Geometry Dash:", e)
				}
			}
			c.mu.Unlock()
			_ = server.Shutdown(context.Background())
		}
	}()
	fmt.Println("GeoCheats:", url)
	if !background {
		if e = openURL(url); e != nil {
			fmt.Println("Open this URL in your browser:", url)
		}
	}
	if e = server.Serve(listener); e != nil && e != http.ErrServerClosed {
		fmt.Println(e)
	}
}
