package router

import (
	"strings"

	"github.com/Areontar/flatline/internal/specialists"
)

func Route(category string) specialists.Profile {
	key := normalize(category)
	if p, ok := specialists.All[key]; ok {
		return p
	}
	return specialists.All["generic"]
}

func normalize(c string) string {
	c = strings.ToLower(strings.TrimSpace(c))
	switch {
	case strings.Contains(c, "web"):
		return "web"
	case strings.Contains(c, "recon") || strings.Contains(c, "enum") || strings.Contains(c, "net"):
		return "recon"
	case strings.Contains(c, "foren") || strings.Contains(c, "steg"):
		return "forensics-stego"
	case strings.Contains(c, "pwn") || strings.Contains(c, "rev") || strings.Contains(c, "bin"):
		return "pwn-rev"
	case strings.Contains(c, "ad") || strings.Contains(c, "active") || strings.Contains(c, "windows") || strings.Contains(c, "domain"):
		return "ad-windows"
	case strings.Contains(c, "prompt") || strings.Contains(c, "inject") || strings.Contains(c, "jailbreak") || strings.Contains(c, "llm") || strings.Contains(c, "chatbot"):
		return "prompt-attack"
	case strings.Contains(c, "password"), strings.Contains(c, "crack"), strings.Contains(c, "hash"), strings.Contains(c, "brute"), strings.Contains(c, "credential"):
		return "password"
	default:
		return "generic"
	}
}
