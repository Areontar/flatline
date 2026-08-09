package router

import (
	"testing"

	"github.com/Areontar/flatline/internal/specialists"
)

func TestRouteNormalizes(t *testing.T) {
	cases := map[string]string{
		"Web":               "web",
		"forensics/steg":    "forensics-stego",
		"PWN":               "pwn-rev",
		"active-directory":  "ad-windows",
		"recon":             "recon",
		"prompt-injection":  "prompt-attack",
		"LLM jailbreak":     "prompt-attack",
		"password-cracking": "password",
		"crypto":            "generic",
	}
	for in, wantKey := range cases {
		got := Route(in)
		want := specialists.All[wantKey]
		if got != want {
			t.Fatalf("Route(%q): wrong specialist (want key %q StartRung %d, got StartRung %d)", in, wantKey, want.StartRung, got.StartRung)
		}
	}
}
