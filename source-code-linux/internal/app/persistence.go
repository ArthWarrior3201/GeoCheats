package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"geocheats/internal/core"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type settings struct {
	Version         int               `json:"version"`
	Enabled         map[string]string `json:"enabled"`
	LoginStart      bool              `json:"loginStart"`
	LoginConfigured bool              `json:"loginConfigured"`
}

func atomicFile(path string, data []byte) error {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".geocheats-*")
	if e != nil {
		return e
	}
	temp := f.Name()
	defer os.Remove(temp)
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(data)
	}
	if e == nil {
		e = f.Sync()
	}
	e = errors.Join(e, f.Close())
	if e != nil {
		return e
	}
	return os.Rename(temp, path)
}

func (c *controller) loadSettings() {
	c.enabled = map[string]string{}
	for _, name := range []string{"settings.json", "settings.backup.json"} {
		b, e := os.ReadFile(filepath.Join(c.root, name))
		if e != nil {
			continue
		}
		var s settings
		if json.Unmarshal(b, &s) != nil || s.Version != 1 || s.Enabled == nil {
			continue
		}
		c.enabled = s.Enabled
		c.loginStart = s.LoginStart
		c.loginConfigured = s.LoginConfigured
		if name != "settings.json" {
			c.message = "Recovered remembered switches from backup."
		}
		return
	}
}

func (c *controller) saveSettings() error {
	b, e := json.MarshalIndent(settings{Version: 1, Enabled: c.enabled, LoginStart: c.loginStart, LoginConfigured: c.loginConfigured}, "", "  ")
	if e != nil {
		return e
	}
	// Two complete, atomically replaced copies; damaged primary can recover on restart.
	if e = atomicFile(filepath.Join(c.root, "settings.json"), b); e != nil {
		return e
	}
	if e = atomicFile(filepath.Join(c.root, "settings.backup.json"), b); e != nil {
		c.message = "Switch saved; backup could not be updated: " + e.Error()
	}
	return nil
}

func (c *controller) setEnabled(id string, enabled bool) error {
	p, ok := c.mods[id]
	if !ok {
		return errors.New("unknown mod")
	}
	if enabled {
		if c.image == nil || c.image.SHA256 != p.Target {
			return errors.New("link the matching game build first")
		}
		for other, target := range c.enabled {
			if q, ok := c.mods[other]; ok && target == p.Target && other != id && p.Group != "" && p.Group == q.Group {
				return fmt.Errorf("turn off %s first", q.Name)
			}
		}
	}
	old := c.enabled[id]
	loginWarning := ""
	if c.session != nil {
		var e error
		if enabled {
			e = c.session.Enable(p)
		} else {
			e = c.session.Disable(id)
		}
		if e != nil {
			return e
		}
	}
	if enabled && !c.loginConfigured {
		if e := configureLogin(true); e != nil {
			loginWarning = "; login start unavailable: " + e.Error()
		} else {
			c.loginStart = true
			c.loginConfigured = true
		}
	}
	if enabled {
		c.enabled[id] = p.Target
	} else {
		delete(c.enabled, id)
	}
	if e := c.saveSettings(); e != nil {
		if old == "" {
			delete(c.enabled, id)
		} else {
			c.enabled[id] = old
		}
		if c.session != nil {
			if old == "" {
				_ = c.session.Disable(id)
			} else {
				_ = c.session.Enable(p)
			}
		}
		return fmt.Errorf("could not save switch: %w", e)
	}
	c.suspended = false
	c.nextConnect = time.Time{}
	c.message = p.Name + map[bool]string{true: " remembered ON", false: " OFF"}[enabled]
	if enabled && c.session == nil {
		c.message += "; waiting for the linked game"
	}
	c.message += loginWarning
	return nil
}

func (c *controller) restoreEnabled() {
	ids := make([]string, 0, len(c.enabled))
	for id := range c.enabled {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		p, ok := c.mods[id]
		if !ok || c.image == nil || c.enabled[id] != p.Target || p.Target != c.image.SHA256 {
			continue
		}
		if _, on := c.session.Active[id]; on {
			continue
		}
		if e := c.session.Resume(p); e != nil {
			c.message = p.Name + " remembered, but inactive: " + e.Error()
		}
	}
}

func (c *controller) maintain(now time.Time) {
	if c.session != nil && !c.session.Mem.Alive() {
		_ = c.session.Close()
		c.session = nil
		c.chord = false
		c.triggerUntil = time.Time{}
	}
	if now.Before(c.nextConnect) {
		return
	}
	c.nextConnect = now.Add(2 * time.Second)
	if c.suspended || len(c.enabled) == 0 || c.image == nil {
		return
	}
	if c.session == nil {
		if list, e := core.FindProcesses(c.path); e == nil && len(list) == 0 {
			c.message = "Waiting for Geometry Dash."
			return
		}
		// Re-inspection in Connect rejects an updated game before opening its memory.
		if e := c.action("connect", request{}); e != nil {
			c.message = "Remembered switches waiting: " + e.Error()
			return
		}
	} else {
		c.restoreEnabled()
	}
}

func (c *controller) prepareMacMod() {
	delete(c.mods, "geo.mac.invincibility")
	if c.image == nil || c.image.Format != "macho" {
		return
	}
	p, e := core.MacInvincibility(c.path)
	if e != nil {
		c.message = "This Mac game build needs a compatible mod profile."
		return
	}
	c.mods[p.ID] = p
	c.message = "Game linked."
}

type speedSnapshot struct {
	PID     int                `json:"pid"`
	Base    uint64             `json:"base"`
	Hash    string             `json:"hash"`
	Records []core.SpeedRecord `json:"records"`
}

func (c *controller) sessionJournal() error {
	s := c.session
	info := s.Mem.Info()
	path := filepath.Join(c.root, "speed-recovery.json")
	s.SpeedJournal = func(records []core.SpeedRecord) error {
		b, e := json.Marshal(speedSnapshot{info.PID, info.Base, s.Image.SHA256, records})
		if e != nil {
			return e
		}
		return atomicFile(path, b)
	}
	if b, e := os.ReadFile(path); e == nil {
		var snapshot speedSnapshot
		if json.Unmarshal(b, &snapshot) != nil {
			return errors.New("damaged speed recovery file; restart Geometry Dash before removing speed-recovery.json")
		}
		if snapshot.PID == info.PID && snapshot.Base == info.Base && snapshot.Hash == s.Image.SHA256 {
			if e := s.RecoverSpeeds(snapshot.Records); e != nil {
				return e
			}
		}
	}
	return s.SpeedJournal(nil)
}
