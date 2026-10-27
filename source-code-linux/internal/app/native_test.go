//go:build !darwin

package app

import (
	"encoding/binary"
	"encoding/json"
	"fyne.io/fyne/v2/test"
	"geocheats/internal/core"
	"os"
	"path/filepath"
	"testing"
)

func fakeGame(t *testing.T) string {
	t.Helper()
	b := make([]byte, 1024)
	put16 := func(p int, v uint16) { binary.LittleEndian.PutUint16(b[p:], v) }
	put32 := func(p int, v uint32) { binary.LittleEndian.PutUint32(b[p:], v) }
	copy(b, "MZ")
	put32(0x3c, 128)
	copy(b[128:], "PE\x00\x00")
	put16(132, 0x8664)
	put16(134, 1)
	put16(148, 240)
	put16(150, 0x22)
	put16(152, 0x20b)
	binary.LittleEndian.PutUint64(b[176:], 0x140000000)
	put32(184, 4096)
	put32(188, 512)
	put32(208, 8192)
	put32(212, 512)
	put32(260, 16)
	copy(b[392:], ".text")
	put32(400, 256)
	put32(404, 4096)
	put32(408, 512)
	put32(412, 512)
	put32(428, 0x60000020)
	b[512] = 0x55
	path := filepath.Join(t.TempDir(), "GeometryDash.exe")
	if e := os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
	return path
}

func TestLinkHidesChooserAndEnableWhileGameClosed(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("GeoCheats")
	s, e := newService(t.TempDir(), false)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	s.c.path = ""
	s.c.image = nil
	s.c.loginConfigured = true
	ui := buildControls(w, s, nil)
	if !ui.prompt.Visible() || ui.linked.Visible() {
		t.Fatal("unlinked UI did not show choose-file prompt")
	}
	game := fakeGame(t)
	if e = s.Link(game); e != nil {
		t.Fatal(e)
	}
	ui.render()
	if ui.prompt.Visible() || !ui.linked.Visible() {
		t.Fatal("chooser did not disappear after linking")
	}
	im, e := core.Inspect(game)
	if e != nil {
		t.Fatal(e)
	}
	p := core.Package{Schema: 2, ID: "test.invincibility", Name: "Invincibility", Target: im.SHA256, Format: "pe", Arch: "amd64", Kind: "patch", Patches: []core.Patch{{RVA: "1000", Expected: "55", Replacement: "c3"}}}
	raw, _ := json.Marshal(p)
	if e = s.Import(raw); e != nil {
		t.Fatal(e)
	}
	ui.render()
	check := ui.checks[p.ID]
	if check == nil || check.Disabled() {
		t.Fatal("compatible switch disabled while game is closed")
	}
	check.SetChecked(true)
	if s.Snapshot().Connected {
		t.Fatal("test unexpectedly connected to a running game")
	}
	if s.c.enabled[p.ID] != im.SHA256 {
		t.Fatal("closed-game switch was not saved")
	}
	manual := s.Snapshot()
	auto := &controller{root: t.TempDir()}
	auto.load()
	auto.autoLinkFrom([]string{game})
	if auto.image == nil || auto.path != manual.Path {
		t.Fatal("auto-link and manual link did not produce the same linked game")
	}
}

func TestSteamLibraryManifestAndBothMacBundleNames(t *testing.T) {
	root := t.TempDir()
	extra := t.TempDir()
	if e := os.MkdirAll(filepath.Join(root, "steamapps"), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(root, "steamapps", "libraryfolders.vdf"), []byte(`"libraryfolders" { "1" { "path" "`+filepath.ToSlash(extra)+`" } }`), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.MkdirAll(filepath.Join(extra, "steamapps"), 0700); e != nil {
		t.Fatal(e)
	}
	_ = os.WriteFile(filepath.Join(extra, "steamapps", "appmanifest_322170.acf"), []byte(`"installdir" "Custom GD Folder"`), 0600)
	for _, name := range []string{"GeometryDash.app", "Geometry Dash.app"} {
		bundle := filepath.Join(extra, "steamapps", "common", "Custom GD Folder", name)
		dir := filepath.Join(bundle, "Contents", "MacOS")
		if e := os.MkdirAll(dir, 0700); e != nil {
			t.Fatal(e)
		}
		_ = os.WriteFile(filepath.Join(dir, "GeometryDash"), []byte("fixture"), 0700)
		resolved, e := resolve(bundle)
		if e != nil || filepath.Base(resolved) != "GeometryDash" {
			t.Fatal("bundle did not resolve", resolved, e)
		}
	}
	found := discoverRoots([]string{root})
	if len(found) != 2 {
		t.Fatalf("expected both bundle variants in additional Steam library, got %v", found)
	}
}

func TestRejectedToggleReturnsToSavedState(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("GeoCheats")
	s, e := newService(t.TempDir(), false)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	s.c.image = &core.Image{SHA256: core.KnownHash, Format: "pe", Arch: "amd64"}
	s.c.loginConfigured = true
	s.c.enabled["geo.super-fast"] = core.KnownHash
	ui := buildControls(w, s, nil)
	w.SetContent(ui.root)
	ui.checks["geo.super-slow"].SetChecked(true)
	if ui.checks["geo.super-slow"].Checked {
		t.Fatal("a rejected toggle still looked enabled")
	}
	if s.c.enabled["geo.super-slow"] != "" {
		t.Fatal("rejected preference was saved")
	}
}
