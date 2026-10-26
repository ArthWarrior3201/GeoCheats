package core

import "embed"

//go:embed mods/*.dash
var bundled embed.FS

func Builtins() map[string]Package {
	result := map[string]Package{}
	entries, _ := bundled.ReadDir("mods")
	for _, entry := range entries {
		raw, e := bundled.ReadFile("mods/" + entry.Name())
		if e != nil {
			continue
		}
		p, e := Decode(raw)
		if e == nil {
			result[p.ID] = p
		}
	}
	return result
}
