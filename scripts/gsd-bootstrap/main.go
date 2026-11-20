// Command gsd-bootstrap creates local GSD metadata for this checkout.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	profileFile  = ".gsd/workspace-profile.yaml"
	manifestFile = ".gsd/harness-manifest.yaml"
	configFile   = ".gsd/config.yaml"
	bundleFile   = ".bin/gsd-assets-v1.6.0.zip"
)

type artifact struct {
	Path      string `json:"path"`
	Primitive int    `json:"primitive"`
	Template  string `json:"template"`
	Checksum  string `json:"checksum,omitempty"`
	Note      string `json:"note,omitempty"`
}

type manifest struct {
	SchemaVersion string            `json:"schema_version"`
	Version       string            `json:"gsd_version"`
	InstalledAt   string            `json:"installed_at"`
	Home          string            `json:"gsd_home"`
	ProfileHash   string            `json:"profile_hash"`
	ConfigHash    string            `json:"config_hash"`
	Preset        string            `json:"install_preset,omitempty"`
	PrimaryPack   string            `json:"primary_stack_pack,omitempty"`
	StackPacks    []string          `json:"stack_packs,omitempty"`
	Layers        []string          `json:"install_layers,omitempty"`
	Packs         []string          `json:"capability_packs,omitempty"`
	Overlays      []json.RawMessage `json:"capability_pack_overlays,omitempty"`
	Primitives    []int             `json:"primitives_installed"`
	Artifacts     []artifact        `json:"artifacts"`
	Variables     map[string]string `json:"variables_used"`
}

type seed struct {
	BundleHash string         `json:"bundle_sha256"`
	Profile    map[string]any `json:"profile"`
	Manifest   manifest       `json:"manifest"`
}

func main() {
	dir, err := os.Getwd()
	if err == nil {
		err = generate(dir)
	}
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err) // Exit unsuccessfully even when stderr is unavailable.
		os.Exit(1)
	}
}

func generate(dir string) (err error) {
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return fmt.Errorf("resolve checkout: %w", err)
	}
	if !filepath.IsAbs(dir) || real != filepath.Clean(dir) {
		return errors.New("checkout must be an absolute path without symlinks or reparse points")
	}
	root, err := os.OpenRoot(real)
	if err != nil {
		return fmt.Errorf("open checkout: %w", err)
	}
	defer func() { err = errors.Join(err, root.Close()) }()

	missing := 0
	for _, path := range []string{profileFile, manifestFile} {
		err := checkFile(root, path)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			missing++
		case err != nil:
			return err
		}
	}
	if missing == 0 {
		return nil
	}
	if missing != 2 {
		return errors.New("incomplete local metadata; preserve the existing file and restore its matching counterpart")
	}
	return render(root, real)
}

func render(root *os.Root, dir string) error {
	raw, err := readFile(root, ".gsd/checkout-seed.json")
	if err != nil {
		return err
	}
	var s seed
	if err := json.Unmarshal(raw, &s); err != nil {
		return fmt.Errorf("decode checkout seed: %w", err)
	}
	if s.Profile["schema_version"] != "1.1.0" || s.Manifest.SchemaVersion != "1.1.0" ||
		s.Manifest.Version != "1.6.0" || len(s.Manifest.Artifacts) == 0 {
		return errors.New("checkout seed must describe GSD 1.6.0 with profile and manifest schema 1.1.0")
	}
	bundle, err := readFile(root, bundleFile)
	if err != nil {
		return err
	}
	if actual := fmt.Sprintf("%x", sha256.Sum256(bundle)); actual != s.BundleHash {
		return fmt.Errorf("workspace-local bundle %s does not match the pinned SHA-256", bundleFile)
	}
	cfg, err := readFile(root, configFile)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	s.Profile["workspace_path"] = dir
	s.Profile["generated_at"] = now
	profile, err := encode(s.Profile)
	if err != nil {
		return err
	}
	m := s.Manifest
	m.InstalledAt = now
	m.Home = filepath.Join(dir, filepath.FromSlash(bundleFile))
	m.ProfileHash = "sha256:" + digest(profile)
	m.ConfigHash = "sha256:" + digest(cfg)
	if m.Variables == nil {
		m.Variables = make(map[string]string)
	}
	m.Variables["DATE"] = now[:len(time.DateOnly)]
	m.Variables["WORKSPACE_ROOT"] = dir
	m.Variables["GSD_BUNDLE_PATH"] = bundleFile
	m.Variables["GSD_BUNDLE_VERSION"] = m.Version
	m.Variables["GSD_BUNDLE_SHA256"] = s.BundleHash
	m.Variables["GSD_RELEASE_TAG"] = "v" + m.Version
	if err := hashArtifacts(root, &m, profile); err != nil {
		return err
	}
	index, err := encode(m)
	if err != nil {
		return err
	}
	if err := createFile(root, profileFile, profile); err != nil {
		return err
	}
	return createFile(root, manifestFile, index)
}

func hashArtifacts(root *os.Root, m *manifest, profile []byte) error {
	seen := make(map[string]bool)
	for i := range m.Artifacts {
		a := &m.Artifacts[i]
		if !portableArtifact(a.Path) || seen[strings.ToLower(a.Path)] {
			return fmt.Errorf("invalid or duplicate portable artifact path %q", a.Path)
		}
		seen[strings.ToLower(a.Path)] = true
		if a.Primitive < 1 || a.Primitive > 10 || a.Template == "" {
			return fmt.Errorf("invalid artifact metadata for %s", a.Path)
		}
		raw := profile
		if a.Path != profileFile {
			var err error
			raw, err = readFile(root, a.Path)
			if err != nil {
				return err
			}
		}
		a.Checksum = digest(raw)
	}
	return nil
}

func portableArtifact(path string) bool {
	if !fs.ValidPath(path) || !filepath.IsLocal(path) || strings.ContainsAny(path, `\:`) {
		return false
	}
	switch path {
	case "AGENTS.md", "start.ps1", "start.sh", configFile, profileFile, ".gsd/backlog-registry.yaml",
		".github/copilot-instructions.md", ".markdownlint.json":
		return true
	}
	for _, prefix := range []string{".github/agents/", ".github/instructions/", ".github/policies/", ".github/prompts/", ".github/skills/"} {
		if strings.HasPrefix(path, prefix) && strings.HasSuffix(path, ".md") {
			return true
		}
	}
	return strings.HasPrefix(path, "scripts/") && (strings.HasSuffix(path, ".sh") || strings.HasSuffix(path, ".ps1"))
}

func checkFile(root *os.Root, path string) error {
	parts := strings.Split(path, "/")
	for i := range parts {
		name := strings.Join(parts[:i+1], "/")
		info, err := root.Lstat(name)
		if err != nil {
			return fmt.Errorf("inspect %s: %w", name, err)
		}
		if (i == len(parts)-1 && !info.Mode().IsRegular()) ||
			(i != len(parts)-1 && info.Mode().Type() != fs.ModeDir) {
			return fmt.Errorf("%s is not a regular checkout file or directory", name)
		}
	}
	return nil
}

func readFile(root *os.Root, path string) ([]byte, error) {
	if err := checkFile(root, path); err != nil {
		return nil, err
	}
	raw, err := root.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return raw, nil
}

func createFile(root *os.Root, path string, raw []byte) error {
	file, err := root.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	_, writeErr := file.Write(raw)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func digest(raw []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))))
}

func encode(value any) ([]byte, error) {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode checkout metadata: %w", err)
	}
	return append(raw, '\n'), nil
}
