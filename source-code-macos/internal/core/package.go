package core

import (
	"crypto/sha256"
	"debug/macho"
	"debug/pe"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type Guard struct {
	RVA      string `json:"rva"`
	Expected string `json:"expectedHex"`
}
type Patch struct {
	RVA         string `json:"rva"`
	Expected    string `json:"expectedHex"`
	Replacement string `json:"replacementHex"`
}
type Package struct {
	Schema      int     `json:"schema"`
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Target      string  `json:"targetSha256"`
	Format      string  `json:"format"`
	Arch        string  `json:"arch"`
	Kind        string  `json:"kind"`
	Effect      string  `json:"effect,omitempty"`
	Factor      float64 `json:"factor,omitempty"`
	Group       string  `json:"group,omitempty"`
	Status      string  `json:"status"`
	Guards      []Guard `json:"guards,omitempty"`
	Patches     []Patch `json:"patches,omitempty"`
}
type Image struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Format string `json:"format"`
	Arch   string `json:"arch"`
	Size   int64  `json:"size"`
	Base   uint64 `json:"preferredBase"`
}

func Offset(s string) (uint64, error) {
	return strconv.ParseUint(strings.TrimPrefix(strings.ToLower(s), "0x"), 16, 64)
}
func Bytes(s string) ([]byte, error) {
	return hex.DecodeString(strings.Join(strings.Fields(strings.ReplaceAll(s, "-", " ")), ""))
}
func (p Package) Validate() error {
	if p.Schema != 2 || p.ID == "" || len(p.ID) > 100 || p.Name == "" || len(p.Name) > 160 {
		return errors.New("invalid schema or mod metadata")
	}
	h, e := hex.DecodeString(p.Target)
	if e != nil || len(h) != 32 {
		return errors.New("an exact executable SHA-256 is required")
	}
	if (p.Format != "pe" && p.Format != "macho") || (p.Arch != "amd64" && p.Arch != "arm64") {
		return errors.New("packages require a PE or thin Mach-O image with an exact amd64/arm64 architecture")
	}
	if p.Kind != "patch" && p.Kind != "field" && p.Kind != "hotkey" {
		return errors.New("unknown mod kind")
	}
	if p.Kind == "field" {
		if p.Effect != "speed" && p.Effect != "high-jump" && p.Effect != "fly" {
			return errors.New("unknown field effect")
		}
		if p.Effect == "speed" && (p.Factor < 0.1 || p.Factor > 5) {
			return errors.New("speed factor must be between 0.1 and 5")
		}
	}
	if p.Kind != "field" && len(p.Patches) == 0 {
		return errors.New("mod contains no patch")
	}
	if len(p.Patches) > 32 || len(p.Guards) > 32 {
		return errors.New("too many patches or guards")
	}
	for _, g := range p.Guards {
		r, e := Offset(g.RVA)
		b, be := Bytes(g.Expected)
		if e != nil || r == 0 || be != nil || len(b) < 4 || len(b) > 256 {
			return errors.New("invalid signature guard")
		}
	}
	for _, v := range p.Patches {
		r, e := Offset(v.RVA)
		a, ae := Bytes(v.Expected)
		b, be := Bytes(v.Replacement)
		if e != nil || r == 0 || ae != nil || be != nil || len(a) == 0 || len(a) != len(b) || len(a) > 256 {
			return errors.New("patch byte lengths must match and be 1-256 bytes")
		}
	}
	return nil
}
func Decode(raw []byte) (Package, error) {
	var p Package
	if len(raw) > 128*1024 {
		return p, errors.New("package exceeds 128 KiB")
	}
	e := json.Unmarshal(raw, &p)
	if e != nil {
		return p, e
	}
	return p, p.Validate()
}
func Inspect(path string) (Image, error) {
	var im Image
	im.Path = path
	f, e := os.Open(path)
	if e != nil {
		return im, e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		return im, e
	}
	if !st.Mode().IsRegular() {
		return im, errors.New("choose an executable file, not a folder")
	}
	im.Size = st.Size()
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return im, e
	}
	im.SHA256 = hex.EncodeToString(h.Sum(nil))
	if pf, e := pe.Open(path); e == nil {
		defer pf.Close()
		im.Format = "pe"
		if pf.Machine == pe.IMAGE_FILE_MACHINE_AMD64 {
			im.Arch = "amd64"
		} else {
			im.Arch = fmt.Sprintf("pe-%x", pf.Machine)
		}
		if oh, ok := pf.OptionalHeader.(*pe.OptionalHeader64); ok {
			im.Base = oh.ImageBase
		}
		return im, nil
	}
	if mf, e := macho.Open(path); e == nil {
		defer mf.Close()
		im.Format = "macho"
		if mf.Cpu == macho.CpuArm64 {
			im.Arch = "arm64"
		} else if mf.Cpu == macho.CpuAmd64 {
			im.Arch = "amd64"
		} else {
			im.Arch = strings.ToLower(mf.Cpu.String())
		}
		if seg := mf.Segment("__TEXT"); seg != nil {
			im.Base = seg.Addr
		}
		return im, nil
	}
	if mf, e := macho.OpenFat(path); e == nil {
		defer mf.Close()
		im.Format = "macho-universal"
		im.Arch = "universal"
		return im, nil
	}
	return im, errors.New("file is not a supported PE or Mach-O executable")
}
func ReadRVA(path string, rva uint64, count int) ([]byte, error) {
	if count <= 0 || count > 4096 {
		return nil, errors.New("invalid read size")
	}
	if mf, e := macho.Open(path); e == nil {
		defer mf.Close()
		text := mf.Segment("__TEXT")
		if text == nil {
			return nil, errors.New("Mach-O has no __TEXT segment")
		}
		address := text.Addr + rva
		for _, load := range mf.Loads {
			if seg, ok := load.(*macho.Segment); ok && address >= seg.Addr && address+uint64(count) <= seg.Addr+seg.Filesz {
				b, e := seg.Data()
				if e != nil {
					return nil, e
				}
				start := address - seg.Addr
				return b[start : start+uint64(count)], nil
			}
		}
		return nil, errors.New("offset is outside Mach-O file-backed segments")
	}
	f, e := pe.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	for _, s := range f.Sections {
		if rva >= uint64(s.VirtualAddress) && rva+uint64(count) <= uint64(s.VirtualAddress)+uint64(s.Size) {
			data, e := s.Data()
			if e != nil {
				return nil, e
			}
			i := rva - uint64(s.VirtualAddress)
			return data[i : i+uint64(count)], nil
		}
	}
	return nil, errors.New("RVA is outside file-backed sections")
}
func VerifyFile(p Package, path string) error {
	if e := p.Validate(); e != nil {
		return e
	}
	im, e := Inspect(path)
	if e != nil {
		return e
	}
	if im.SHA256 != p.Target || im.Format != p.Format || im.Arch != p.Arch {
		return errors.New("mod does not match this executable")
	}
	for _, g := range p.Guards {
		r, _ := Offset(g.RVA)
		expected, _ := Bytes(g.Expected)
		b, e := ReadRVA(path, r, len(expected))
		if e != nil {
			return e
		}
		if !Equal(b, expected) {
			return fmt.Errorf("file signature mismatch at %s", g.RVA)
		}
	}
	for _, g := range p.Patches {
		r, _ := Offset(g.RVA)
		expected, _ := Bytes(g.Expected)
		b, e := ReadRVA(path, r, len(expected))
		if e != nil {
			return e
		}
		if !Equal(b, expected) {
			return fmt.Errorf("file patch signature mismatch at %s", g.RVA)
		}
	}
	return nil
}
func Equal(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
