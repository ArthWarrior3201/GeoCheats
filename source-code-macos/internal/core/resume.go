package core

import "errors"

type originalView struct {
	Memory
	changes []applied
}

func (m originalView) Read(a uint64, n int) ([]byte, error) {
	b, e := m.Memory.Read(a, n)
	if e != nil {
		return nil, e
	}
	for _, p := range m.changes {
		for i, v := range p.original {
			pos := p.address + uint64(i)
			if pos >= a && pos < a+uint64(n) {
				b[pos-a] = v
			}
		}
	}
	return b, nil
}

// Resume adopts a fully applied remembered code patch after a controller crash.
// It never accepts a partially applied patch, arbitrary bytes, or a mismatched file.
func (s *Session) Resume(p Package) (err error) {
	if _, ok := s.Active[p.ID]; ok {
		return nil
	}
	if p.Kind == "field" {
		return s.Enable(p)
	}
	if e := VerifyFile(p, s.Image.Path); e != nil {
		return e
	}
	resume, e := s.Mem.Pause()
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, resume()) }()
	var changes []applied
	originals, replacements := 0, 0
	for _, patch := range p.Patches {
		r, e := Offset(patch.RVA)
		if e != nil {
			return e
		}
		before, _ := Bytes(patch.Expected)
		after, _ := Bytes(patch.Replacement)
		if r+uint64(len(before)) > s.Mem.Info().Size {
			return errors.New("remembered patch outside main module")
		}
		a := s.Mem.Info().Base + r
		b, e := s.Mem.Read(a, len(before))
		if e != nil {
			return e
		}
		if Equal(b, before) {
			originals++
		} else if Equal(b, after) {
			replacements++
		} else {
			return errors.New("remembered patch has unexpected live bytes; restart the game")
		}
		for _, existing := range s.changes {
			for _, c := range existing {
				if a < c.address+uint64(len(c.original)) && c.address < a+uint64(len(before)) {
					return errors.New("remembered patch overlaps another active mod")
				}
			}
		}
		changes = append(changes, applied{a, before, after})
	}
	if originals == len(p.Patches) {
		// Avoid nesting task suspension / ptrace when Enable applies the patch.
		if e := resume(); e != nil {
			return e
		}
		resume = func() error { return nil }
		return s.Enable(p)
	}
	if replacements != len(p.Patches) {
		return errors.New("partially applied remembered patch; restart the game")
	}
	for _, other := range s.Active {
		if p.Group != "" && p.Group == other.Group {
			return errors.New("remembered mod conflicts with an active mod")
		}
	}
	real := s.Mem
	s.Mem = originalView{real, changes}
	e = s.Check(p)
	s.Mem = real
	if e != nil {
		return e
	}
	s.changes[p.ID] = changes
	s.Active[p.ID] = p
	if p.Kind == "hotkey" {
		if e := resume(); e != nil {
			return e
		}
		resume = func() error { return nil }
		return s.restore(p.ID)
	}
	return nil
}
