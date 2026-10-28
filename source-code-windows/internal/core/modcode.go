package core

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

// ParseModCode translates the portable ModCode 1 patch subset into a Dash recipe.
// The host supplies the inspected target and validates all offsets and bytes.
func ParseModCode(source string, image Image) (Package, error) {
	p := Package{Schema: 2, Target: image.SHA256, Format: image.Format, Arch: image.Arch, Kind: "patch", Status: "Unverified gameplay"}
	scanner := bufio.NewScanner(strings.NewReader(source))
	line := 0
	header := false
	for scanner.Scan() {
		line++
		s := strings.TrimSpace(scanner.Text())
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		if !header {
			if s != "modcode 1" {
				return p, fmt.Errorf("line %d: expected modcode 1", line)
			}
			header = true
			continue
		}
		fields := strings.Fields(s)
		switch fields[0] {
		case "name", "id":
			raw := strings.TrimSpace(strings.TrimPrefix(s, fields[0]))
			v, e := strconv.Unquote(raw)
			if e != nil || v == "" {
				return p, fmt.Errorf("line %d: use a quoted value", line)
			}
			if fields[0] == "name" {
				p.Name = v
			} else {
				p.ID = v
			}
		case "patch":
			// patch 0xRVA "expected bytes" -> "replacement bytes"
			rest := strings.TrimSpace(strings.TrimPrefix(s, "patch"))
			space := strings.IndexAny(rest, " \t")
			if space < 0 {
				return p, fmt.Errorf("line %d: missing patch bytes", line)
			}
			rva := rest[:space]
			parts := strings.SplitN(strings.TrimSpace(rest[space:]), "->", 2)
			if len(parts) != 2 {
				return p, fmt.Errorf("line %d: expected ->", line)
			}
			before, e := strconv.Unquote(strings.TrimSpace(parts[0]))
			if e != nil {
				return p, fmt.Errorf("line %d: invalid expected bytes", line)
			}
			after, e := strconv.Unquote(strings.TrimSpace(parts[1]))
			if e != nil {
				return p, fmt.Errorf("line %d: invalid replacement bytes", line)
			}
			p.Patches = append(p.Patches, Patch{RVA: rva, Expected: before, Replacement: after})
		default:
			return p, fmt.Errorf("line %d: unknown instruction %q", line, fields[0])
		}
	}
	if e := scanner.Err(); e != nil {
		return p, e
	}
	if !header {
		return p, fmt.Errorf("missing modcode 1 header")
	}
	if len(p.Patches) == 0 {
		return p, fmt.Errorf("add at least one patch")
	}
	if e := p.Validate(); e != nil {
		return p, e
	}
	return p, nil
}
