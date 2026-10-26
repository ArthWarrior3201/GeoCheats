package core

import (
	"debug/pe"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

func Scan(path, pattern string) ([]string, error) {
	words := strings.Fields(pattern)
	if len(words) < 4 || len(words) > 256 {
		return nil, errors.New("patterns must be 4-256 space-separated bytes or ?? wildcards")
	}
	needle := make([]byte, len(words))
	mask := make([]bool, len(words))
	known := 0
	for i, w := range words {
		if w == "??" || w == "?" {
			continue
		}
		b, e := hex.DecodeString(w)
		if e != nil || len(b) != 1 {
			return nil, fmt.Errorf("invalid pattern byte %q", w)
		}
		needle[i] = b[0]
		mask[i] = true
		known++
	}
	if known < 4 {
		return nil, errors.New("at least four concrete bytes required")
	}
	f, e := pe.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	result := []string{}
	for _, section := range f.Sections {
		if section.Characteristics&0x20000000 == 0 {
			continue
		}
		data, e := section.Data()
		if e != nil {
			return nil, e
		}
		for i := 0; i+len(needle) <= len(data); i++ {
			ok := true
			for j := range needle {
				if mask[j] && data[i+j] != needle[j] {
					ok = false
					break
				}
			}
			if ok {
				result = append(result, fmt.Sprintf("0x%X", uint64(section.VirtualAddress)+uint64(i)))
				if len(result) > 10000 {
					return nil, errors.New("too many matches; use a more specific signature")
				}
			}
		}
	}
	return result, nil
}
