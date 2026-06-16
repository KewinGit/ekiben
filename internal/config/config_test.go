package config

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultInterval(t *testing.T) {
	if Default().Interval() != 2*time.Second {
		t.Fatalf("default interval = %v", Default().Interval())
	}
}

func TestLoadMissingWritesDefault(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yml")
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Theme != "dark" {
		t.Fatalf("theme = %q want dark", c.Theme)
	}
	// file should now exist and reload identically
	c2, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c2.Theme != "dark" || len(c2.CardFields) != len(c.CardFields) {
		t.Fatalf("reload mismatch: %+v vs %+v", c, c2)
	}
}

func TestRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yml")
	c := Default()
	c.GroupOrder = []string{"hydra", "arya"}
	if err := c.Save(p); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.GroupOrder) != 2 || got.GroupOrder[0] != "hydra" {
		t.Fatalf("group order not persisted: %+v", got.GroupOrder)
	}
}
