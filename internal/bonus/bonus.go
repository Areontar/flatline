package bonus

import (
	"context"
	"os"
	"regexp"
	"strings"

	"github.com/Areontar/flatline/internal/loop"
)

var flagRe = regexp.MustCompile(`(?i)(flag|ctf|hal)\{[^}]{1,120}\}`)
var flagEnvKeyRe = regexp.MustCompile(`(?i)^flag(_.+)?$`)

var wellKnown = []string{"/flag", "/flag.txt", "/bonus", "/bonus.txt", "/root/flag.txt", "/tmp/flag.txt"}

func Scan(environ []string, readFile func(string) (string, error)) []string {
	var out []string
	seen := map[string]bool{}
	add := func(s string) {
		for _, m := range flagRe.FindAllString(s, -1) {
			if !seen[m] {
				seen[m] = true
				out = append(out, m)
			}
		}
	}
	addRaw := func(s string) {
		s = strings.TrimSpace(s)
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}

	// First pass: scan env var names for BONUS_FLAG or FLAG_* pattern (raw values)
	for _, kv := range environ {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			key := kv[:i]
			value := kv[i+1:]

			// Check if key is BONUS_FLAG or matches FLAG(_.+)?
			if strings.EqualFold(key, "BONUS_FLAG") || flagEnvKeyRe.MatchString(key) {
				addRaw(value)
			}
		}
	}

	// Second pass: regex-based scanning of env values
	for _, kv := range environ {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			add(kv[i+1:])
		}
	}

	// Scan well-known files
	for _, p := range wellKnown {
		if body, err := readFile(p); err == nil {
			add(body)
		}
	}
	return out
}

func Grab(ctx context.Context, sub loop.Submitter, environ []string) bool {
	read := func(p string) (string, error) {
		b, err := os.ReadFile(p)
		return string(b), err
	}
	for _, cand := range Scan(environ, read) {
		if res, err := sub.SubmitFlag(ctx, cand); err == nil && (res.Correct || res.AlreadySolved) {
			return true
		}
	}
	return false
}
