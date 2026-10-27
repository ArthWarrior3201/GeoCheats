package app

import (
	"encoding/json"
	"errors"
	"geocheats/internal/core"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type ModView struct {
	Package                     core.Package
	Enabled, Active, Compatible bool
}
type View struct {
	Linked, Connected, LoginStart bool
	Path, Message                 string
	Mods                          []ModView
}
type Service struct {
	c       *controller
	done    chan struct{}
	monitor chan struct{}
	once    sync.Once
	wake    chan struct{}
	unlock  func()
}

func newService(root string, lock bool) (*Service, error) {
	var unlock func()
	if lock {
		var e error
		unlock, e = instanceLock(root)
		if errors.Is(e, errAlreadyRunning) {
			stopped, upgradeErr := retireLegacy(root)
			if upgradeErr != nil {
				return nil, upgradeErr
			}
			if stopped {
				for attempt := 0; attempt < 20; attempt++ {
					time.Sleep(100 * time.Millisecond)
					unlock, e = instanceLock(root)
					if e == nil {
						break
					}
				}
			}
		}
		if errors.Is(e, errAlreadyRunning) {
			_ = atomicFile(filepath.Join(root, "show-window"), []byte("show"))
			return nil, e
		}
		if e != nil {
			return nil, e
		}
	}
	c := &controller{root: root, message: ""}
	c.load()
	c.autoLink()
	if c.loginStart {
		if e := configureLogin(true); e != nil {
			c.message = "Could not refresh the login shortcut."
		}
	}
	_ = os.Remove(filepath.Join(root, "running.json"))
	return &Service{c: c, done: make(chan struct{}), wake: make(chan struct{}, 1), unlock: unlock}, nil
}

func (c *controller) autoLink() {
	c.autoLinkFrom(discover())
}
func (c *controller) autoLinkFrom(candidates []string) {
	if c.image != nil {
		return
	}
	for _, path := range candidates {
		if c.action("link", request{Path: path}) == nil {
			return
		}
	}
	c.path = ""
	c.image = nil
	c.message = ""
}

func (s *Service) Snapshot() View {
	c := s.c
	c.mu.Lock()
	defer c.mu.Unlock()
	v := View{Linked: c.image != nil, Path: c.path, Message: c.message, LoginStart: c.loginStart}
	v.Connected = c.session != nil && c.session.Mem.Alive()
	for _, p := range c.mods {
		active := false
		if c.session != nil {
			_, active = c.session.Active[p.ID]
		}
		v.Mods = append(v.Mods, ModView{p, c.enabled[p.ID] == p.Target, active, c.image != nil && c.image.SHA256 == p.Target})
	}
	sort.Slice(v.Mods, func(i, j int) bool { return v.Mods[i].Package.Name < v.Mods[j].Package.Name })
	return v
}

func (s *Service) action(name string, r request) error {
	s.c.mu.Lock()
	defer s.c.mu.Unlock()
	e := s.c.action(name, r)
	if e != nil {
		s.c.message = e.Error()
	}
	return e
}
func (s *Service) Link(path string) error { return s.action("link", request{Path: path}) }
func (s *Service) Toggle(id string, on bool) error {
	return s.action("toggle", request{ID: id, Enabled: on})
}
func (s *Service) Login(on bool) error { return s.action("login-start", request{Enabled: on}) }
func (s *Service) Launch() error       { return s.action("launch", request{}) }
func (s *Service) Trigger() error      { return s.action("trigger", request{}) }
func (s *Service) Flight(direction int) error {
	return s.action("flight", request{Direction: direction})
}
func (s *Service) Import(data []byte) error {
	if !json.Valid(data) {
		return errors.New("invalid .dash file")
	}
	return s.action("import", request{Package: data})
}

func (s *Service) Start() {
	s.monitor = make(chan struct{})
	go func() {
		defer close(s.monitor)
		ticker := time.NewTicker(8 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-s.done:
				return
			case now := <-ticker.C:
				c := s.c
				c.mu.Lock()
				c.maintain(now)
				if session := c.session; session != nil {
					chord, flight := core.PlatformKeys(session.Mem.Info().PID)
					if core.GlobalKeysAvailable() {
						session.SetFlight(flight)
					}
					if chord && !c.chord {
						if e := session.Trigger(); e == nil {
							c.triggerUntil = now.Add(500 * time.Millisecond)
						} else {
							c.message = e.Error()
						}
					}
					c.chord = chord
					if !c.triggerUntil.IsZero() && now.After(c.triggerUntil) {
						if e := session.ClearTriggers(); e == nil {
							c.triggerUntil = time.Time{}
						} else {
							c.message = e.Error()
						}
					}
					if e := session.Tick(); e != nil {
						c.message = e.Error()
					}
				}
				c.mu.Unlock()
				if _, e := os.Stat(filepath.Join(c.root, "show-window")); e == nil {
					_ = os.Remove(filepath.Join(c.root, "show-window"))
					select {
					case s.wake <- struct{}{}:
					default:
					}
				}
			}
		}
	}()
}

func (s *Service) Close() error {
	var result error
	s.once.Do(func() {
		close(s.done)
		if s.monitor != nil {
			<-s.monitor
		}
		s.c.mu.Lock()
		defer s.c.mu.Unlock()
		if s.c.session != nil {
			result = s.c.session.Close()
			s.c.session = nil
		}
		if s.unlock != nil {
			s.unlock()
		}
	})
	return result
}
