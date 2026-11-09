package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveHostedTokenWrites0600AndLoadTrimsWhitespace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := SaveHostedToken(path, "  abc.def.ghi  \n"); err != nil {
		t.Fatalf("SaveHostedToken: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat token: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("token permissions = %v, want 0600", got)
	}
	token, err := LoadHostedToken(path)
	if err != nil {
		t.Fatalf("LoadHostedToken: %v", err)
	}
	if token != "abc.def.ghi" {
		t.Fatalf("token = %q", token)
	}
}

func TestSaveHostedTokenChmodsExistingPermissiveFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatalf("seed token: %v", err)
	}
	if err := SaveHostedToken(path, "new-token"); err != nil {
		t.Fatalf("SaveHostedToken: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat token: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("token permissions = %v, want 0600", got)
	}
}

func TestDeleteHostedTokenIgnoresMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-token")
	if err := DeleteHostedToken(path); err != nil {
		t.Fatalf("DeleteHostedToken missing file: %v", err)
	}
}
