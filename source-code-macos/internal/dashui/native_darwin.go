package dashui

import (
	"encoding/json"
	"fmt"
	loader "geocheats/internal/app"
	"geocheats/internal/cocoaui"
	"geocheats/internal/core"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func Run() {
	game := ""
	var image core.Image
	var advanced *core.Package
	draft := map[string]string{"name": "", "id": "", "rva": "", "expected": "", "replacement": ""}
	message := ""
	config, _ := os.UserConfigDir()
	saved := filepath.Join(config, "GeoCheatsDash", "linked-game.txt")
	link := func(path string) error {
		p, e := loader.ResolveExecutable(path)
		if e != nil {
			return e
		}
		im, e := core.Inspect(p)
		if e != nil {
			return e
		}
		game = p
		image = im
		advanced = nil
		_ = os.MkdirAll(filepath.Dir(saved), 0700)
		_ = os.WriteFile(saved, []byte(p), 0600)
		return nil
	}
	if b, e := os.ReadFile(saved); e == nil {
		_ = link(string(b))
	}
	if game == "" {
		for _, path := range loader.InstalledGames() {
			if link(path) == nil {
				break
			}
		}
	}
	state := func(refresh bool) map[string]any {
		return map[string]any{"linked": game != "", "draft": draft, "message": message, "refresh": refresh}
	}
	use := func(p core.Package) {
		draft["name"] = p.Name
		draft["id"] = p.ID
		if len(p.Patches) > 0 {
			draft["rva"] = p.Patches[0].RVA
			draft["expected"] = p.Patches[0].Expected
			draft["replacement"] = p.Patches[0].Replacement
		}
		advanced = &p
	}
	recipe := func(input map[string]any) core.Package {
		for key := range draft {
			if value, ok := input[key].(string); ok {
				if (key == "rva" || key == "expected" || key == "replacement") && value != draft[key] {
					advanced = nil
				}
				draft[key] = value
			}
		}
		p := core.Package{Schema: 2, ID: draft["id"], Name: draft["name"], Target: image.SHA256, Format: image.Format, Arch: image.Arch, Kind: "patch", Status: "Unverified gameplay", Patches: []core.Patch{{RVA: draft["rva"], Expected: draft["expected"], Replacement: draft["replacement"]}}}
		if advanced != nil {
			p = *advanced
			p.ID = draft["id"]
			p.Name = draft["name"]
		}
		if p.Target == "AUTO" {
			p.Target = image.SHA256
		}
		if p.Format == "AUTO" {
			p.Format = image.Format
		}
		if p.Arch == "AUTO" {
			p.Arch = image.Arch
		}
		return p
	}
	cocoaui.Run("Dash", func(action string, input map[string]any) map[string]any {
		text := func(key string) string { s, _ := input[key].(string); return s }
		var e error
		refresh := false
		switch action {
		case "state", "quit":
			return state(false)
		case "link":
			e = link(text("path"))
			refresh = true
		case "import":
			var b []byte
			b, e = os.ReadFile(text("path"))
			if e == nil {
				var p core.Package
				e = json.Unmarshal(b, &p)
				if e == nil {
					use(p)
					refresh = true
					message = "Recipe opened."
				}
			}
		case "advanced":
			var p core.Package
			e = json.Unmarshal([]byte(text("json")), &p)
			if e == nil {
				use(p)
				refresh = true
			}
		case "recipe":
			p := recipe(input)
			b, _ := json.MarshalIndent(p, "", "  ")
			return map[string]any{"json": string(b)}
		case "validate", "save":
			p := recipe(input)
			e = core.VerifyFile(p, game)
			if e == nil {
				if action == "validate" {
					return map[string]any{"filename": regexp.MustCompile(`[^a-zA-Z0-9_.-]`).ReplaceAllString(p.ID, "_") + ".dash"}
				}
				path := text("path")
				if !strings.EqualFold(filepath.Ext(path), ".dash") {
					e = fmt.Errorf("save with a .dash extension")
				} else {
					var b []byte
					b, e = json.MarshalIndent(p, "", "  ")
					if e == nil {
						e = os.WriteFile(path, append(b, '\n'), 0644)
					}
					if e == nil {
						message = "Saved " + filepath.Base(path)
					}
				}
			}
		}
		if e != nil {
			return map[string]any{"error": e.Error()}
		}
		return state(refresh)
	})
}
