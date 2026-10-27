package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMacBundleExecutableUsesBinaryPlist(t *testing.T) {
	bundle := filepath.Join(t.TempDir(), "Geometry Dash.app")
	macos := filepath.Join(bundle, "Contents", "MacOS")
	if e := os.MkdirAll(macos, 0700); e != nil {
		t.Fatal(e)
	}
	expected := filepath.Join(macos, "GDMain")
	if e := os.WriteFile(expected, []byte("fixture"), 0700); e != nil {
		t.Fatal(e)
	}
	_ = os.WriteFile(filepath.Join(macos, "GeometryHelper"), []byte("wrong executable"), 0700)
	plist := filepath.Join(bundle, "Contents", "Info.plist")
	xml := `<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>CFBundleExecutable</key><string>GDMain</string></dict></plist>`
	if e := os.WriteFile(plist, []byte(xml), 0600); e != nil {
		t.Fatal(e)
	}
	if out, e := exec.Command("/usr/bin/plutil", "-convert", "binary1", plist).CombinedOutput(); e != nil {
		t.Fatal(e, string(out))
	}
	result, e := resolve(bundle)
	if e != nil || result != expected {
		t.Fatalf("binary bundle metadata not respected: %q %v", result, e)
	}
}
