package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateFreshAndIdempotent(t *testing.T) {
	dir := bootstrapFixture(t, "AGENTS.md")
	if err := generate(dir); err != nil {
		t.Fatal(err)
	}
	profile := fixtureRead(t, dir, profileFile)
	manifest := fixtureRead(t, dir, manifestFile)
	var p struct {
		Workspace string `json:"workspace_path"`
	}
	if err := json.Unmarshal(profile, &p); err != nil || p.Workspace != dir {
		t.Fatalf("profile must identify the checkout root: %v", err)
	}
	var m struct {
		ProfileHash string            `json:"profile_hash"`
		ConfigHash  string            `json:"config_hash"`
		Variables   map[string]string `json:"variables_used"`
		Artifacts   []struct {
			Checksum string `json:"checksum"`
		} `json:"artifacts"`
	}
	if err := json.Unmarshal(manifest, &m); err != nil {
		t.Fatal(err)
	}
	if m.ProfileHash != fmt.Sprintf("sha256:%x", sha256.Sum256(profile)) ||
		m.ConfigHash != fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("config\n"))) {
		t.Fatal("manifest does not record the hashes of the generated profile and checked-out config")
	}
	if len(m.Artifacts) != 1 || m.Artifacts[0].Checksum != fmt.Sprintf("%x", sha256.Sum256([]byte("agent\n"))) {
		t.Fatal("manifest does not record the portable artifact checksum")
	}
	if m.Variables["GSD_RELEASE_TAG"] != "v1.6.0" {
		t.Fatal("manifest does not record the pinned bundle release tag")
	}
	if err := generate(dir); err != nil {
		t.Fatal(err)
	}
	if string(profile) != string(fixtureRead(t, dir, profileFile)) || string(manifest) != string(fixtureRead(t, dir, manifestFile)) {
		t.Fatal("rerun changed existing metadata")
	}
}

func TestGeneratePreservesIncompleteLocalPair(t *testing.T) {
	dir := bootstrapFixture(t, "AGENTS.md")
	fixtureWrite(t, dir, profileFile, "operator-owned")
	if err := generate(dir); err == nil || !strings.Contains(err.Error(), "incomplete local metadata") {
		t.Fatalf("want incomplete metadata error, got %v", err)
	}
	if string(fixtureRead(t, dir, profileFile)) != "operator-owned" {
		t.Fatal("operator profile changed")
	}
	requireAbsent(t, dir, manifestFile)
}

func TestGenerateRejectsPrivateAndEscapingArtifact(t *testing.T) {
	for _, path := range []string{"../outside.md", ".env.local", ".gsd/carry-forward.yaml"} {
		t.Run(path, func(t *testing.T) {
			dir := bootstrapFixture(t, path)
			if err := generate(dir); err == nil || !strings.Contains(err.Error(), "portable artifact path") {
				t.Fatalf("want portable artifact path error, got %v", err)
			}
			requireAbsent(t, dir, profileFile)
			requireAbsent(t, dir, manifestFile)
		})
	}
}

func TestGenerateRejectsArtifactSymlink(t *testing.T) {
	dir := bootstrapFixture(t, "scripts/link.sh")
	target := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(target, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "scripts", "link.sh")
	if err := os.Mkdir(filepath.Dir(link), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := generate(dir); err == nil || !strings.Contains(err.Error(), "not a regular checkout file") {
		t.Fatalf("want symlink refusal, got %v", err)
	}
	requireAbsent(t, dir, profileFile)
	requireAbsent(t, dir, manifestFile)
}

func bootstrapFixture(t *testing.T, artifact string) string {
	t.Helper()
	dir := t.TempDir()
	fixtureWrite(t, dir, ".bin/gsd-assets-v1.6.0.zip", "fixture")
	fixtureWrite(t, dir, ".gsd/config.yaml", "config\r\n")
	fixtureWrite(t, dir, "AGENTS.md", "agent\n")
	fixtureWrite(t, dir, ".gsd/checkout-seed.json", fmt.Sprintf(`{
  "bundle_sha256": "%x",
  "profile": {"schema_version": "1.1.0", "languages": {}, "build": {}, "test": {}},
  "manifest": {
    "schema_version": "1.1.0",
    "gsd_version": "1.6.0",
    "primitives_installed": [9],
    "artifacts": [{"path": %q, "primitive": 9, "template": "foundation/AGENTS.md.tmpl"}]
  }
}`, sha256.Sum256([]byte("fixture")), artifact))
	return dir
}

func fixtureWrite(t *testing.T, dir, path, value string) {
	t.Helper()
	path = filepath.Join(dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}

func fixtureRead(t *testing.T, dir, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func requireAbsent(t *testing.T, dir, path string) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(path))); !os.IsNotExist(err) {
		t.Fatalf("%s should still be absent, got %v", path, err)
	}
}
