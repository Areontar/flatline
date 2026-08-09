package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSkill(t *testing.T, dir, name, content string) {
	t.Helper()
	skillDir := filepath.Join(dir, name)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestCatalog(t *testing.T) {
	tmp := t.TempDir()
	writeSkill(t, tmp, "web-enum", "---\nname: web-enum\ndescription: Brute-force dirs and files with ffuf against a URL.\n---\n# body...")
	writeSkill(t, tmp, "stego-triage", "# Stego Triage\nRun file, strings, binwalk, exiftool over an artifact.")

	got := Catalog(tmp)

	if !strings.HasPrefix(got, "Reusable skills are installed") {
		t.Fatalf("catalog does not start with expected header: %q", got)
	}
	if !strings.Contains(got, "web-enum") {
		t.Fatalf("catalog missing web-enum: %q", got)
	}
	if !strings.Contains(got, "Brute-force dirs and files with ffuf against a URL.") {
		t.Fatalf("catalog missing web-enum description: %q", got)
	}
	if !strings.Contains(got, "stego-triage") {
		t.Fatalf("catalog missing stego-triage: %q", got)
	}
	if !strings.Contains(got, "Run file, strings, binwalk, exiftool over an artifact.") {
		t.Fatalf("catalog missing stego-triage summary: %q", got)
	}
}

func TestCatalogSkipsDirsWithoutSkillMd(t *testing.T) {
	tmp := t.TempDir()
	writeSkill(t, tmp, "web-enum", "---\ndescription: Find hidden dirs.\n---\n")
	// A bundled data directory (e.g. wordlists) with no SKILL.md must NOT appear.
	if err := os.MkdirAll(filepath.Join(tmp, "wordlists"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "wordlists", "rockyou-small.txt"), []byte("password\n123456\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := Catalog(tmp)
	if !strings.Contains(got, "web-enum") {
		t.Fatalf("catalog missing real skill web-enum: %q", got)
	}
	if strings.Contains(got, "wordlists") {
		t.Fatalf("catalog should not list the wordlists data dir (no SKILL.md): %q", got)
	}
}

func TestCatalogMissingDir(t *testing.T) {
	if got := Catalog("/nonexistent-xyz"); got != "" {
		t.Fatalf("expected empty catalog for missing dir, got %q", got)
	}
}

func TestCatalogEmptyDir(t *testing.T) {
	tmp := t.TempDir()
	if got := Catalog(tmp); got != "" {
		t.Fatalf("expected empty catalog for empty dir, got %q", got)
	}
}
