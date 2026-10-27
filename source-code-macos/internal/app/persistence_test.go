package app

import (
	"geocheats/internal/core"
	"os"
	"path/filepath"
	"testing"
)

func TestRememberDisableAndBackupRecovery(t *testing.T) {
	root := t.TempDir()
	c := &controller{root: root}
	c.load()
	c.image = &core.Image{SHA256: core.KnownHash}
	c.loginConfigured = true
	if e := c.setEnabled("geo.invincibility", true); e != nil {
		t.Fatal(e)
	}
	next := &controller{root: root}
	next.load()
	if next.enabled["geo.invincibility"] != core.KnownHash {
		t.Fatal("enabled switch lost across restart")
	}
	if e := os.WriteFile(filepath.Join(root, "settings.json"), []byte("broken"), 0600); e != nil {
		t.Fatal(e)
	}
	next.loadSettings()
	if next.enabled["geo.invincibility"] != core.KnownHash {
		t.Fatal("backup recovery failed")
	}
	if e := next.setEnabled("geo.invincibility", false); e != nil {
		t.Fatal(e)
	}
	final := &controller{root: root}
	final.load()
	if len(final.enabled) != 0 {
		t.Fatal("disabled switch reappeared")
	}
}

func TestRememberedConflictsAndMismatchedBuild(t *testing.T) {
	c := &controller{root: t.TempDir()}
	c.load()
	c.loginConfigured = true
	c.image = &core.Image{SHA256: core.KnownHash}
	if e := c.setEnabled("geo.super-fast", true); e != nil {
		t.Fatal(e)
	}
	if e := c.setEnabled("geo.super-slow", true); e == nil {
		t.Fatal("conflicting speeds remembered")
	}
	c.image = &core.Image{SHA256: "different"}
	if e := c.setEnabled("geo.invincibility", true); e == nil {
		t.Fatal("different build accepted")
	}
	// Turning off an old remembered mod is allowed even with a different linked game.
	if e := c.setEnabled("geo.super-fast", false); e != nil {
		t.Fatal(e)
	}
}
