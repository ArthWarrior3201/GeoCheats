package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"geocheats/internal/core"
	"os"
	"path/filepath"
	"strings"
)

func fail(e error) { fmt.Fprintln(os.Stderr, "Dash:", e); os.Exit(1) }
func output(v any) { b, _ := json.MarshalIndent(v, "", "  "); fmt.Println(string(b)) }
func help() {
	fmt.Print(`Dash v1 - GeoCheats mod authoring tool (no Geode)

  dash inspect GAME              Identify SHA-256, architecture and file format
  dash read GAME RVA COUNT       Read file-backed PE bytes (RVA is hexadecimal)
  dash scan GAME "48 8B ?? 18"    Search PE executable sections for a byte pattern
  dash validate GAME MOD         Verify package fingerprint and expected bytes
  dash build --game GAME --recipe MOD.json --out MOD.dash
  dash make --game GAME --id author.mod --name "Name" --rva 0x1234
            --expected "48 89 ..." --replace "C3 90 ..." --out MOD.dash
  dash export-mods DIRECTORY     Export all seven built-in recipes
  dash mac-profile GAME OUT.dash Discover experimental Intel Mac invincibility

Dash never guesses addresses and never changes the game executable.
Read mod_building_guide.txt before making a recipe.
`)
}
func main() {
	if len(os.Args) < 2 {
		help()
		return
	}
	args := os.Args[2:]
	switch os.Args[1] {
	case "mac-profile":
		if len(args) != 2 {
			fail(errors.New("usage: dash mac-profile GAME OUT.dash"))
		}
		p, e := core.MacInvincibility(args[0])
		if e != nil {
			fail(e)
		}
		if e = core.VerifyFile(p, args[0]); e != nil {
			fail(e)
		}
		writePackage(args[1], p)
		fmt.Println("Created exact-file Intel Mac recipe. Gameplay and process permissions remain unverified.")
	case "inspect":
		if len(args) != 1 {
			fail(errors.New("usage: dash inspect GAME"))
		}
		im, e := core.Inspect(args[0])
		if e != nil {
			fail(e)
		}
		output(im)
	case "read":
		if len(args) != 3 {
			fail(errors.New("usage: dash read GAME RVA COUNT"))
		}
		r, e := core.Offset(args[1])
		if e != nil {
			fail(e)
		}
		var n int
		if _, e = fmt.Sscan(args[2], &n); e != nil {
			fail(e)
		}
		b, e := core.ReadRVA(args[0], r, n)
		if e != nil {
			fail(e)
		}
		fmt.Println(strings.ToUpper(hex.EncodeToString(b)))
	case "validate":
		if len(args) != 2 {
			fail(errors.New("usage: dash validate GAME MOD"))
		}
		raw, e := os.ReadFile(args[1])
		if e != nil {
			fail(e)
		}
		p, e := core.Decode(raw)
		if e != nil {
			fail(e)
		}
		if e = core.VerifyFile(p, args[0]); e != nil {
			fail(e)
		}
		fmt.Println("Verified file fingerprint and all expected bytes. Gameplay is not proven by this check.")
	case "export-mods":
		if len(args) != 1 {
			fail(errors.New("usage: dash export-mods DIRECTORY"))
		}
		if e := os.MkdirAll(args[0], 0755); e != nil {
			fail(e)
		}
		for _, p := range core.Builtins() {
			writePackage(filepath.Join(args[0], p.ID+".dash"), p)
		}
	case "build", "make":
		fs := flag.NewFlagSet("dash "+os.Args[1], flag.ExitOnError)
		game := fs.String("game", "", "game executable")
		recipe := fs.String("recipe", "", "recipe JSON")
		out := fs.String("out", "", "output .dash")
		id := fs.String("id", "", "mod ID")
		name := fs.String("name", "", "name")
		rva := fs.String("rva", "", "RVA")
		expected := fs.String("expected", "", "expected bytes")
		replace := fs.String("replace", "", "replacement bytes")
		_ = fs.Parse(args)
		if *game == "" || *out == "" {
			fail(errors.New("--game and --out are required"))
		}
		im, e := core.Inspect(*game)
		if e != nil {
			fail(e)
		}
		var p core.Package
		if os.Args[1] == "build" {
			raw, e := os.ReadFile(*recipe)
			if e != nil {
				fail(e)
			}
			if e = json.Unmarshal(raw, &p); e != nil {
				fail(e)
			}
			if p.Target == "AUTO" {
				p.Target = im.SHA256
			}
		} else {
			p = core.Package{Schema: 2, ID: *id, Name: *name, Target: im.SHA256, Format: im.Format, Arch: im.Arch, Kind: "patch", Status: "unverified gameplay; file signatures checked", Patches: []core.Patch{{RVA: *rva, Expected: *expected, Replacement: *replace}}}
		}
		if e = core.VerifyFile(p, *game); e != nil {
			fail(e)
		}
		writePackage(*out, p)
		fmt.Println("Built", *out, "for", im.SHA256)
	case "scan":
		if len(args) != 2 {
			fail(errors.New("usage: dash scan GAME PATTERN"))
		}
		matches, e := core.Scan(args[0], args[1])
		if e != nil {
			fail(e)
		}
		output(matches)
	case "help", "--help", "-h":
		help()
	default:
		help()
		os.Exit(1)
	}
}
func writePackage(path string, p core.Package) {
	b, _ := json.MarshalIndent(p, "", "  ")
	if _, e := os.Stat(path); e == nil {
		fail(fmt.Errorf("output exists: %s; choose a new path", path))
	}
	if e := os.WriteFile(path, append(b, '\n'), 0644); e != nil {
		fail(e)
	}
}
