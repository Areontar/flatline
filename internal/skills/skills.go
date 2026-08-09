// Package skills reads reusable playbook "skills" that a container may ship
// under /skills/<name>/SKILL.md and turns them into a concise, prompt-ready
// catalog so a weak model can discover and use them without extra tool calls.
package skills

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const maxSummaryLen = 140

// Catalog reads skill directories under dir (each a subdirectory containing a
// SKILL.md) and returns a concise, prompt-ready catalog: one line per skill with
// its name and a short summary. Returns "" if dir is missing, empty, or has no
// readable skills - so it degrades to nothing when no skills are installed.
func Catalog(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	names := make([]string, 0, len(entries))
	summaries := make(map[string]string, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		skillPath := filepath.Join(dir, name, "SKILL.md")
		if _, err := os.Stat(skillPath); err != nil {
			// Not a skill (no SKILL.md) - e.g. a bundled data/wordlist dir. Skip
			// it so it doesn't pollute the catalog as a phantom skill.
			continue
		}
		summary := summarize(skillPath)
		names = append(names, name)
		summaries[name] = summary
	}
	if len(names) == 0 {
		return ""
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString("Reusable skills are installed. When a task matches one, read /skills/<name>/SKILL.md and run it:")
	for _, name := range names {
		summary := summaries[name]
		if summary == "" {
			b.WriteString("\n- " + name)
		} else {
			b.WriteString("\n- " + name + ": " + summary)
		}
	}
	return b.String()
}

// summarize derives a short summary for a SKILL.md file. It returns "" if the
// file is missing, unreadable, or empty.
func summarize(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	content := string(data)
	lines := strings.Split(content, "\n")

	var summary string
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		// YAML frontmatter block: look for description:, fall back to name:.
		var description, name string
		for i := 1; i < len(lines); i++ {
			line := lines[i]
			if strings.TrimSpace(line) == "---" {
				break
			}
			if v, ok := frontmatterValue(line, "description:"); ok {
				description = v
			} else if v, ok := frontmatterValue(line, "name:"); ok {
				name = v
			}
		}
		if description != "" {
			summary = description
		} else {
			summary = name
		}
	} else {
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if strings.HasPrefix(trimmed, "#") {
				continue
			}
			summary = trimmed
			break
		}
	}

	summary = strings.TrimSpace(summary)
	if len(summary) > maxSummaryLen {
		summary = strings.TrimSpace(summary[:maxSummaryLen])
	}
	return summary
}

// frontmatterValue checks if line starts with key (after trimming leading
// whitespace) and returns the trimmed value after it.
func frontmatterValue(line, key string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, key) {
		return "", false
	}
	v := strings.TrimSpace(strings.TrimPrefix(trimmed, key))
	v = strings.Trim(v, `"'`)
	return v, true
}
