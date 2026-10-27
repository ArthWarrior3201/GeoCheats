package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"geocheats/internal/core"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"time"
)

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
	return map[string]any{"name": "GeoCheats", "version": "1.3.0-experimental", "platform": runtime.GOOS + "/" + runtime.GOARCH, "image": c.image, "path": c.path, "connected": connected, "pid": pid, "mods": mods, "message": c.message, "detected": discover(), "globalKeys": core.GlobalKeysAvailable(), "loginStart": c.loginStart}
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
		c.message = "Game linked."
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
