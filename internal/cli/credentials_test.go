package cli

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

type fakeStore struct {
	token         string
	err           error
	reads, writes int
}

func (s *fakeStore) Get() (string, error) { s.reads++; return s.token, s.err }
func (s *fakeStore) Set(token string) error {
	s.writes++
	if s.err != nil {
		return s.err
	}
	s.token = token
	return nil
}

func TestCredentialSourceSelection(t *testing.T) {
	t.Setenv("AUTODL_TOKEN", "")
	s := &fakeStore{token: "keyring-token"}
	o := options{store: s}
	token, err := o.resolveToken()
	if err != nil || token != "keyring-token" || s.reads != 1 {
		t.Fatalf("keyring not selected: %q %v", token, err)
	}
	s.err = errors.New("backend unavailable")
	t.Setenv("AUTODL_TOKEN", "environment-token")
	token, err = o.resolveToken()
	if err != nil || token != "environment-token" || s.reads != 1 {
		t.Fatal("environment should bypass keyring")
	}
	t.Setenv("AUTODL_TOKEN", "")
	o.configPath = filepath.Join(t.TempDir(), "missing.json")
	if _, err = o.resolveToken(); err == nil || s.reads != 1 {
		t.Fatal("explicit config should bypass keyring")
	}
	o.configPath = ""
	if _, err = o.resolveToken(); err == nil || !strings.Contains(err.Error(), "keyring unavailable") {
		t.Fatalf("backend failure: %v", err)
	}
}

func TestSetTokenUsesKeyringAndDoesNotEchoSecret(t *testing.T) {
	for _, fail := range []bool{false, true} {
		s := &fakeStore{}
		if fail {
			s.err = errors.New("locked")
		}
		root := newRoot("test", s)
		root.SetArgs([]string{"config", "set-token"})
		root.SetIn(strings.NewReader("test-secret\n"))
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		err := root.Execute()
		if s.writes != 1 || strings.Contains(out.String(), "test-secret") {
			t.Fatalf("writes=%d output=%s", s.writes, out.String())
		}
		if fail {
			if err == nil || !strings.Contains(err.Error(), "no plaintext file") {
				t.Fatalf("unexpected error: %v", err)
			}
		} else if err != nil || s.token != "test-secret" {
			t.Fatalf("save: %v", err)
		}
	}
}

func TestSetTokenRejectsEmptyAndMultilineInput(t *testing.T) {
	for _, input := range []string{"", "a\nb", strings.Repeat("x", 16385)} {
		s := &fakeStore{}
		root := newRoot("test", s)
		root.SetArgs([]string{"config", "set-token"})
		root.SetIn(strings.NewReader(input))
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		if err := root.Execute(); err == nil || s.writes != 0 {
			t.Fatalf("invalid input saved: %v", err)
		}
	}
}
