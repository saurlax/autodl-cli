package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSaveReplaceAndRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "autodl", "config.json")
	if token, err := Token(path); err != nil || token != "" {
		t.Fatalf("missing config: %s %v", token, err)
	}
	for _, token := range []string{"first", "replacement"} {
		if err := Save(path, token); err != nil {
			t.Fatal(err)
		}
		got, err := Token(path)
		if err != nil || got != token {
			t.Fatalf("read: %s %v", got, err)
		}
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("mode: %v", info.Mode())
		}
	}
	if err := os.WriteFile(path, []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Token(path); err == nil {
		t.Fatal("corrupt config accepted")
	}
}
