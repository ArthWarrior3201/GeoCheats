package core

import (
	"strings"
	"testing"
)

func TestModCodeParsing(t *testing.T) {
	image := Image{SHA256: strings.Repeat("a", 64), Format: "pe", Arch: "amd64"}
	valid := "modcode 1\nname \"Example\"\nid \"author.example\"\npatch 0x1234 \"55\" -> \"C3\"\n"
	p, err := ParseModCode(valid, image)
	if err != nil || len(p.Patches) != 1 || p.Target != image.SHA256 {
		t.Fatalf("unexpected result: %#v %v", p, err)
	}
	for _, bad := range []string{"name \"x\"", strings.Replace(valid, "55\" -> \"C3", "55 00\" -> \"C3", 1), valid + "execute anything\n"} {
		if _, err := ParseModCode(bad, image); err == nil {
			t.Fatalf("accepted invalid input: %s", bad)
		}
	}
}
