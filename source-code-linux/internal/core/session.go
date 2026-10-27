package core

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"time"
)

const KnownHash = "4eee06d5c2e2213b4ba2b1f6233dd426bbdd0e9b29d7c375e499516b6f8f69d5"

type applied struct {
	address               uint64
	original, replacement []byte
}
type playerState struct {
	layer      uint64
	speed      float32
	written    float32
	speedOwned bool
	lastY      float64
	ground     bool
	boosted    time.Time
}
type Session struct {
	Mem          Memory
	Image        Image
	Active       map[string]Package
	changes      map[string][]applied
	players      map[uint64]*playerState
	Pending      *time.Timer
	flight       int
	SpeedJournal func([]SpeedRecord) error
}

func NewSession(m Memory, im Image) *Session {
	return &Session{Mem: m, Image: im, Active: map[string]Package{}, changes: map[string][]applied{}, players: map[uint64]*playerState{}}
}
func (s *Session) Check(p Package) error {
	if e := p.Validate(); e != nil {
		return e
	}
	if p.Format == "macho" && p.Arch == "arm64" {
		return errors.New("ARM64 instruction-cache synchronization is not implemented; native ARM64 code mods are not enabled")
	}
	if !s.Mem.Alive() {
		return errors.New("the game closed; reconnect to a running game")
	}
	if s.Image.SHA256 != p.Target {
		return errors.New("package fingerprint differs from the linked game")
	}
	if s.Image.Format != p.Format || s.Image.Arch != p.Arch {
		return errors.New("package format or CPU differs from the linked game")
	}
	info := s.Mem.Info()
	for _, g := range p.Guards {
		r, _ := Offset(g.RVA)
		b, _ := Bytes(g.Expected)
		if r+uint64(len(b)) > info.Size {
			return errors.New("guard outside main module")
		}
		v, e := s.Mem.Read(info.Base+r, len(b))
		if e != nil {
			return e
		}
		if !Equal(v, b) {
			return fmt.Errorf("live guard mismatch at %s; another mod or a different game build may be active", g.RVA)
		}
	}
	return nil
}
func (s *Session) apply(id string, p Package) (err error) {
	if len(s.changes[id]) != 0 {
		return nil
	}
	resume, e := s.Mem.Pause()
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, resume()) }()
	var plan []applied
	for _, v := range p.Patches {
		r, _ := Offset(v.RVA)
		a := s.Mem.Info().Base + r
		b, _ := Bytes(v.Expected)
		replacement, _ := Bytes(v.Replacement)
		if r+uint64(len(b)) > s.Mem.Info().Size {
			return errors.New("patch outside main module")
		}
		for _, existing := range s.changes {
			for _, c := range existing {
				if a < c.address+uint64(len(c.original)) && c.address < a+uint64(len(b)) {
					return errors.New("overlapping active mods")
				}
			}
		}
		for _, c := range plan {
			if a < c.address+uint64(len(c.original)) && c.address < a+uint64(len(b)) {
				return errors.New("overlapping patches in package")
			}
		}
		actual, e := s.Mem.Read(a, len(b))
		if e != nil {
			return e
		}
		if !Equal(actual, b) {
			return fmt.Errorf("live patch mismatch at %s", v.RVA)
		}
		plan = append(plan, applied{a, b, replacement})
	}
	s.changes[id] = nil
	for _, c := range plan {
		s.changes[id] = append(s.changes[id], c)
		if e = s.Mem.Write(c.address, c.replacement); e != nil {
			err = e
			break
		}
		actual, re := s.Mem.Read(c.address, len(c.replacement))
		if re != nil || !Equal(actual, c.replacement) {
			err = errors.New("patch readback failed")
			break
		}
	}
	if err != nil {
		var rollback error
		for i := len(s.changes[id]) - 1; i >= 0; i-- {
			c := s.changes[id][i]
			rollback = errors.Join(rollback, s.Mem.Write(c.address, c.original))
		}
		if rollback == nil {
			delete(s.changes, id)
		}
		return errors.Join(err, rollback)
	}
	return nil
}
func (s *Session) restore(id string) (err error) {
	list := s.changes[id]
	if len(list) == 0 {
		return nil
	}
	if !s.Mem.Alive() {
		delete(s.changes, id)
		return nil
	}
	resume, e := s.Mem.Pause()
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, resume()) }()
	for _, c := range list {
		b, e := s.Mem.Read(c.address, len(c.replacement))
		if e != nil {
			return e
		}
		if !Equal(b, c.replacement) && !Equal(b, c.original) {
			return errors.New("another mod changed patched bytes; restart the game to reset memory")
		}
	}
	for i := len(list) - 1; i >= 0; i-- {
		c := list[i]
		if e = s.Mem.Write(c.address, c.original); e != nil {
			return e
		}
		b, e := s.Mem.Read(c.address, len(c.original))
		if e != nil || !Equal(b, c.original) {
			return errors.New("restoration readback failed")
		}
	}
	delete(s.changes, id)
	return nil
}
func (s *Session) Enable(p Package) error {
	if _, ok := s.Active[p.ID]; ok {
		return nil
	}
	if e := s.Check(p); e != nil {
		return e
	}
	for _, other := range s.Active {
		if p.Group != "" && p.Group == other.Group {
			return fmt.Errorf("turn off %s first", other.Name)
		}
	}
	if p.Kind == "patch" {
		if e := s.apply(p.ID, p); e != nil {
			return e
		}
	}
	if p.Kind == "field" && p.Target != KnownHash {
		return errors.New("field effects require a supported pointer profile; arbitrary offsets are rejected")
	}
	s.Active[p.ID] = p
	return nil
}
func (s *Session) Disable(id string) error {
	p, ok := s.Active[id]
	if !ok && len(s.changes[id]) == 0 {
		return nil
	}
	if e := s.restore(id); e != nil {
		return e
	}
	if p.Effect == "speed" {
		if e := s.restoreSpeed(); e != nil {
			return e
		}
	}
	delete(s.Active, id)
	return nil
}
func (s *Session) Close() error {
	// Include changes retained after a failed Enable, even when Active was never set.
	for id := range s.changes {
		if e := s.restore(id); e != nil {
			return e
		}
	}
	if e := s.restoreSpeed(); e != nil {
		return e
	}
	s.Active = map[string]Package{}
	return s.Mem.Close()
}
func (s *Session) ptr(a uint64) (uint64, error) {
	b, e := s.Mem.Read(a, 8)
	if e != nil {
		return 0, e
	}
	return binary.LittleEndian.Uint64(b), nil
}
func (s *Session) layer() (uint64, error) {
	gm, e := s.ptr(s.Mem.Info().Base + 0x90bfd0)
	if e != nil || gm == 0 {
		return 0, e
	}
	l, e := s.ptr(gm + 0x230)
	if e != nil || l == 0 {
		return 0, e
	}
	v, e := s.ptr(l)
	if e != nil {
		return 0, e
	}
	if v != s.Mem.Info().Base+0x7d59b8 {
		return 0, errors.New("PlayLayer identity check failed")
	}
	return l, nil
}
func (s *Session) restoreSpeed() error {
	if !s.Mem.Alive() {
		s.players = map[uint64]*playerState{}
		return nil
	}
	owned := false
	for _, state := range s.players {
		owned = owned || state.speedOwned
	}
	if !owned {
		return nil
	}
	layer, e := s.layer()
	if e != nil {
		return e
	}
	for ptr, state := range s.players {
		if !state.speedOwned || state.layer != layer {
			continue
		}
		v, e := s.ptr(ptr)
		if e != nil {
			return e
		}
		if v != s.Mem.Info().Base+0x7d47c8 {
			continue
		}
		b, e := s.Mem.Read(ptr+0x9fc, 4)
		if e != nil {
			return e
		}
		if math.Float32frombits(binary.LittleEndian.Uint32(b)) == state.written {
			binary.LittleEndian.PutUint32(b, math.Float32bits(state.speed))
			if e = s.Mem.Write(ptr+0x9fc, b); e != nil {
				return e
			}
		}
		state.speedOwned = false
	}
	return s.saveSpeeds()
}
func (s *Session) SetFlight(direction int) {
	if direction >= -1 && direction <= 1 {
		s.flight = direction
	}
}
func (s *Session) Tick() error {
	if len(s.Active) == 0 {
		return nil
	}
	if !s.Mem.Alive() {
		s.Active = map[string]Package{}
		s.changes = map[string][]applied{}
		s.players = map[uint64]*playerState{}
		return errors.New("game closed; temporary mods ended")
	}
	hasField := false
	for _, p := range s.Active {
		if p.Kind == "field" {
			hasField = true
		}
	}
	if !hasField {
		return nil
	}
	l, e := s.layer()
	if e != nil {
		return e
	}
	if l == 0 {
		s.players = map[uint64]*playerState{}
		return nil
	}
	// Skip writes when the level is dead/completing. This is an exact-build field guard.
	live, e := s.Mem.Read(l+0x334a, 1)
	if e != nil {
		return e
	}
	if live[0] != 0 {
		return nil
	}
	for _, off := range []uint64{0xdb8, 0xdc0} {
		ptr, e := s.ptr(l + off)
		if e != nil {
			return e
		}
		if ptr == 0 {
			continue
		}
		v, e := s.ptr(ptr)
		if e != nil {
			return e
		}
		if v != s.Mem.Info().Base+0x7d47c8 {
			return errors.New("PlayerObject identity check failed")
		}
		b, e := s.Mem.Read(ptr+0x9a8, 0x58)
		if e != nil {
			return e
		}
		y := math.Float64frombits(binary.LittleEndian.Uint64(b))
		if math.IsNaN(y) || math.IsInf(y, 0) {
			return errors.New("invalid live velocity")
		}
		ground := b[0x9c9-0x9a8] != 0
		inverted := b[0x9c7-0x9a8] != 0
		speed := math.Float32frombits(binary.LittleEndian.Uint32(b[0x9fc-0x9a8:]))
		state := s.players[ptr]
		if state == nil || state.layer != l {
			state = &playerState{layer: l, speed: speed, lastY: y, ground: ground}
			s.players[ptr] = state
		}
		for _, p := range s.Active {
			switch p.Effect {
			case "speed":
				previous := *state
				if !state.speedOwned || speed != state.written {
					state.speed = speed
				}
				wanted := state.speed * float32(p.Factor)
				if math.IsNaN(float64(wanted)) || math.Abs(float64(wanted)) > 20 {
					return errors.New("invalid movement speed")
				}
				out := make([]byte, 4)
				binary.LittleEndian.PutUint32(out, math.Float32bits(wanted))
				state.speedOwned = true
				state.written = wanted
				if !previous.speedOwned || previous.written != wanted || previous.speed != state.speed {
					if e = s.saveSpeeds(); e != nil {
						*state = previous
						return e
					}
				}
				if e = s.Mem.Write(ptr+0x9fc, out); e != nil {
					return e
				}
			case "high-jump":
				sign := 1.0
				if inverted {
					sign = -1
				}
				cube := true
				for i := 0x9c0; i <= 0x9c5; i++ {
					if b[i-0x9a8] != 0 {
						cube = false
					}
				}
				if cube && y*sign > 0.5 && (state.lastY*sign <= 0 || state.ground && !ground) && time.Since(state.boosted) > 150*time.Millisecond {
					out := make([]byte, 8)
					binary.LittleEndian.PutUint64(out, math.Float64bits(y*2.2))
					if e = s.Mem.Write(ptr+0x9a8, out); e != nil {
						return e
					}
					state.boosted = time.Now()
				}
			case "fly":
				out := make([]byte, 8)
				binary.LittleEndian.PutUint64(out, math.Float64bits(float64(s.flight)*6))
				if e = s.Mem.Write(ptr+0x9a8, out); e != nil {
					return e
				}
			}
		}
		state.lastY = y
		state.ground = ground
	}
	return nil
}
func (s *Session) Trigger() error {
	for id, p := range s.Active {
		if p.Kind == "hotkey" {
			l, e := s.layer()
			if e != nil {
				return e
			}
			if l == 0 {
				return errors.New("enter a level before triggering completion")
			}
			ending, e := s.Mem.Read(l+0x3a28, 1)
			if e != nil {
				return e
			}
			if ending[0] != 0 {
				return errors.New("the level ending has already started")
			}
			portal, e := s.ptr(l + 0x3320)
			if e != nil {
				return e
			}
			if portal == 0 {
				return errors.New("this level has no standard end portal; completion mod cannot finish it")
			}
			return s.apply(id, p)
		}
	}
	return errors.New("enable Instant Complete first")
}
func (s *Session) ClearTriggers() error {
	for id, p := range s.Active {
		if p.Kind == "hotkey" {
			if e := s.restore(id); e != nil {
				return e
			}
		}
	}
	return nil
}
