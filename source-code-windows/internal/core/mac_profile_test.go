package core

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// A tiny synthetic Mach-O executable, never a proprietary game fixture.
func macFixture(t *testing.T, symbol string, prologue []byte) string {
	t.Helper()
	b := make([]byte, 1024)
	u32 := func(o int, v uint32) { binary.LittleEndian.PutUint32(b[o:], v) }
	u64 := func(o int, v uint64) { binary.LittleEndian.PutUint64(b[o:], v) }
	const base = 0x100000000
	u32(0, 0xfeedfacf)
	u32(4, 0x1000007)
	u32(8, 3)
	u32(12, 2)
	u32(16, 2)
	u32(20, 176)
	u32(32, 0x19)
	u32(36, 152)
	copy(b[40:], "__TEXT")
	u64(56, base)
	u64(64, 4096)
	u64(80, 1024)
	u32(88, 7)
	u32(92, 5)
	u32(96, 1)
	copy(b[104:], "__text")
	copy(b[120:], "__TEXT")
	u64(136, base+512)
	u64(144, 16)
	u32(152, 512)
	u32(156, 4)
	u32(168, 0x80000400)
	u32(184, 2)
	u32(188, 24)
	u32(192, 640)
	u32(196, 1)
	u32(200, 656)
	u32(204, uint32(len(symbol)+2))
	copy(b[512:], prologue)
	u32(640, 1)
	b[644] = 0xf
	b[645] = 1
	u64(648, base+512)
	copy(b[657:], symbol)
	path := filepath.Join(t.TempDir(), "Geometry Dash")
	if e := os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
	return path
}

func TestMacProfileAndCrashRecovery(t *testing.T) {
	path := macFixture(t, "__ZN9PlayLayer13destroyPlayerEP12PlayerObjectP10GameObject", []byte{0x55, 0x48, 0x89, 0xe5, 0x41, 0x57, 0x41, 0x56})
	p, e := MacInvincibility(path)
	if e != nil {
		t.Fatal(e)
	}
	if e = VerifyFile(p, path); e != nil {
		t.Fatal(e)
	}
	im, e := Inspect(path)
	if e != nil {
		t.Fatal(e)
	}
	m := &fakeMemory{data: map[uint64]byte{}, alive: true}
	b, e := ReadRVA(path, 512, 16)
	if e != nil {
		t.Fatal(e)
	}
	_ = m.Write(m.Info().Base+512, b)
	first := NewSession(m, im)
	if e = first.Enable(p); e != nil {
		t.Fatal(e)
	}
	recovered := NewSession(m, im)
	if e = recovered.Resume(p); e != nil {
		t.Fatal(e)
	}
	if e = recovered.Disable(p.ID); e != nil {
		t.Fatal(e)
	}
	actual, _ := m.Read(m.Info().Base+512, 1)
	if actual[0] != 0x55 {
		t.Fatal("original not restored after adoption")
	}
	// Unrecognized byte changes must never be adopted.
	m.data[m.Info().Base+512] = 0x90
	if e = NewSession(m, im).Resume(p); e == nil {
		t.Fatal("unexpected code accepted")
	}
}

func TestMacDiscoveryRefusesGuesses(t *testing.T) {
	for _, tc := range []struct {
		symbol string
		entry  []byte
	}{
		{"__ZN9OtherName13destroyPlayerEP12PlayerObjectP10GameObject", []byte{0x55, 0x48, 0x89, 0xe5}},
		{"__ZN9PlayLayer13destroyPlayerEP12PlayerObjectP10GameObject", []byte{0x90, 0x90, 0x90, 0x90}},
	} {
		if _, e := MacInvincibility(macFixture(t, tc.symbol, tc.entry)); e == nil {
			t.Fatal("unsupported binary accepted")
		}
	}
}

func TestMacSessionCloseWithoutWindowsFields(t *testing.T) {
	s := NewSession(&fakeMemory{data: map[uint64]byte{}, alive: true}, Image{Format: "macho", Arch: "amd64"})
	if e := s.Close(); e != nil {
		t.Fatal(e)
	}
}
