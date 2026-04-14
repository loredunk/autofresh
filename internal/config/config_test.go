package config

import (
	"path/filepath"
	"testing"
)

func TestConfigRoundTrip(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.json")
	cfg := Config{StartTime: "08:00", Target: "all"}

	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if got.Target != "all" {
		t.Fatalf("got target %q", got.Target)
	}
}
