package core

import (
	"encoding/binary"
	"errors"
	"math"
	"testing"
)

func speedFixture(t *testing.T) (*Session, *fakeMemory, uint64) {
	t.Helper()
	m := &fakeMemory{data: map[uint64]byte{}, alive: true}
	base := m.Info().Base
	put := func(a, v uint64) { b := make([]byte, 8); binary.LittleEndian.PutUint64(b, v); _ = m.Write(a, b) }
	const manager = 0x20000000
	const layer = 0x30000000
	const player = 0x40000000
	put(base+0x90bfd0, manager)
	put(manager+0x230, layer)
	put(layer, base+0x7d59b8)
	put(layer+0xdb8, player)
	put(player, base+0x7d47c8)
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, math.Float32bits(1))
	_ = m.Write(player+0x9fc, b)
	p := Builtins()["geo.super-fast"]
	for _, g := range p.Guards {
		r, _ := Offset(g.RVA)
		b, _ := Bytes(g.Expected)
		_ = m.Write(base+r, b)
	}
	s := NewSession(m, Image{SHA256: KnownHash, Format: "pe", Arch: "amd64"})
	if e := s.Enable(p); e != nil {
		t.Fatal(e)
	}
	return s, m, player
}

func TestSpeedCrashRecoveryDoesNotMultiplyAgain(t *testing.T) {
	s, m, player := speedFixture(t)
	var records []SpeedRecord
	s.SpeedJournal = func(r []SpeedRecord) error { records = append([]SpeedRecord(nil), r...); return nil }
	if e := s.Tick(); e != nil {
		t.Fatal(e)
	}
	if len(records) != 1 {
		t.Fatal("missing speed baseline journal")
	}
	recovered := NewSession(m, s.Image)
	if e := recovered.RecoverSpeeds(records); e != nil {
		t.Fatal(e)
	}
	read := func() float32 {
		b, _ := m.Read(player+0x9fc, 4)
		return math.Float32frombits(binary.LittleEndian.Uint32(b))
	}
	if read() != 1 {
		t.Fatal("baseline not restored")
	}
	if e := recovered.Enable(Builtins()["geo.super-fast"]); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 3; i++ {
		if e := recovered.Tick(); e != nil {
			t.Fatal(e)
		}
	}
	if read() != 4 {
		t.Fatalf("speed compounded to %v", read())
	}
	if e := recovered.Disable("geo.super-fast"); e != nil {
		t.Fatal(e)
	}
	if read() != 1 {
		t.Fatal("disable did not restore speed")
	}
}

func TestSpeedJournalFailurePreventsUnrecoverableWrite(t *testing.T) {
	s, m, player := speedFixture(t)
	s.SpeedJournal = func([]SpeedRecord) error { return errors.New("disk full") }
	if e := s.Tick(); e == nil {
		t.Fatal("journal failure ignored")
	}
	b, _ := m.Read(player+0x9fc, 4)
	if math.Float32frombits(binary.LittleEndian.Uint32(b)) != 1 {
		t.Fatal("speed written before baseline saved")
	}
}
