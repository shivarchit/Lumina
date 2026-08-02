package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"Lumina/pkg/config"
)

func TestSaveToWritesAtomically(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.json")

	cfg := config.Config{Version: 1, IP: "192.168.1.5", Port: "38899", LastColorTemp: 3500}
	if err := config.SaveTo(path, cfg); err != nil {
		t.Fatalf("SaveTo failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading saved config: %v", err)
	}

	var loaded config.Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal saved config: %v", err)
	}
	if loaded.IP != "192.168.1.5" || loaded.Port != "38899" || loaded.LastColorTemp != 3500 {
		t.Fatalf("roundtrip mismatch: %+v", loaded)
	}

	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("temp file left behind after save")
	}
}
