package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunWritesRegistry(t *testing.T) {
	dir := t.TempDir()
	docs := filepath.Join(dir, "docs.md")
	pal := filepath.Join(dir, "palettes.json")
	out := filepath.Join(dir, "tokens.json")
	if err := os.WriteFile(docs, []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pal, []byte(`{"capturedFrom":"CC 1.0.0","palettes":{"dark":{"text":"#fff"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run(docs, pal, out, "https://example.test/docs"); err != nil {
		t.Fatal(err)
	}
	var reg registryFile
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &reg); err != nil || reg.VerifiedAgainst != "CC 1.0.0" || len(reg.Tokens) == 0 {
		t.Fatalf("bad registry: %v %+v", err, reg)
	}
}

func TestRunRejectsBadInputs(t *testing.T) {
	dir := t.TempDir()
	docs := filepath.Join(dir, "docs.md")
	if err := os.WriteFile(docs, []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	badPal := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(badPal, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out.json")
	if err := run(filepath.Join(dir, "missing.md"), badPal, out, "u"); err == nil {
		t.Fatal("missing docs must fail")
	}
	if err := run(docs, badPal, out, "u"); err == nil || !strings.Contains(err.Error(), "palettes") {
		t.Fatalf("malformed palettes must fail: %v", err)
	}
	big := filepath.Join(dir, "big.md")
	if err := os.WriteFile(big, make([]byte, maxDocsBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readLimited(big, maxDocsBytes); err == nil {
		t.Fatal("oversized docs must be rejected")
	}
}
