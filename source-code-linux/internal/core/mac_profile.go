package core

import (
	"debug/macho"
	"encoding/hex"
	"errors"
	"fmt"
)

// Resolve the actual installed binary's symbol rather than guessing Windows RVAs.
// No network is used and the game on disk is never modified.
func MacInvincibility(path string) (Package, error) {
	var p Package
	im, e := Inspect(path)
	if e != nil {
		return p, e
	}
	if im.Format != "macho" || im.Arch != "amd64" {
		return p, errors.New("experimental symbol discovery requires a thin Intel Mach-O game; ARM64/universal game code is not supported")
	}
	f, e := macho.Open(path)
	if e != nil {
		return p, e
	}
	defer f.Close()
	if f.Symtab == nil {
		return p, errors.New("game has no symbol table; an independently verified Mac profile is needed")
	}
	text := f.Segment("__TEXT")
	section := f.Section("__text")
	if text == nil || section == nil {
		return p, errors.New("game has no executable text section")
	}
	const symbol = "__ZN9PlayLayer13destroyPlayerEP12PlayerObjectP10GameObject"
	var address uint64
	matches := 0
	for _, s := range f.Symtab.Syms {
		if s.Name == symbol && s.Type&0xe == 0xe && s.Value >= section.Addr && s.Value+16 <= section.Addr+section.Size {
			address = s.Value
			matches++
		}
	}
	if matches != 1 || address <= text.Addr {
		return p, errors.New("exact PlayLayer::destroyPlayer(PlayerObject*, GameObject*) symbol not uniquely found; no guessed patch was created")
	}
	rva := address - text.Addr
	b, e := ReadRVA(path, rva, 16)
	if e != nil {
		return p, e
	}
	if !Equal(b[:4], []byte{0x55, 0x48, 0x89, 0xe5}) {
		return p, errors.New("Mac death-handler entry does not have the supported Intel frame prologue")
	}
	p = Package{Schema: 2, ID: "geo.mac.invincibility", Name: "Invincibility (Intel Mac experimental)", Description: "Skips the named native death handler. Unverified gameplay; macOS process permissions may prevent activation.", Target: im.SHA256, Format: "macho", Arch: "amd64", Kind: "patch", Status: "Symbol-derived; gameplay unverified", Guards: []Guard{{RVA: fmt.Sprintf("%x", rva+1), Expected: hex.EncodeToString(b[1:])}}, Patches: []Patch{{RVA: fmt.Sprintf("%x", rva), Expected: "55", Replacement: "c3"}}}
	return p, p.Validate()
}
