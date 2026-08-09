package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type Challenge struct{ ID, Slug, Name, Category, Description string }
type Target struct{ IP, Port string }
type Targets map[string]Target // key "" = default target; else namespaced NAME (upper-case)

type Config struct {
	RunID, UserID, TeamUUID     string
	OpenAIBaseURL, MCPEndpoint  string
	MCPHint                     string
	Challenge                   Challenge
	Targets                     Targets
	ModelLadder                 []string
	MaxSteps                    int
}

// defaultLadder is only a fallback used when no model is provided via
// MODEL_LADDER, MODEL, or the platform-injected HAL_AGENT_MODEL. It lists
// REAL HALCTF-available models (smaller-first) so the agent still works if the
// platform ever omits HAL_AGENT_MODEL, instead of dialing a phantom model id.
const defaultLadder = "qwen3.6-35b-a3b,llama-3.1-8b"

func Load(getenv func(string) string) (Config, error) {
	c := Config{
		RunID:         getenv("HAL_RUN_ID"),
		UserID:        getenv("HAL_USER_ID"),
		TeamUUID:      getenv("HAL_TEAM_UUID"),
		OpenAIBaseURL: strings.TrimRight(getenv("OPENAI_BASE_URL"), "/"),
		MCPEndpoint:   getenv("MCP_ENDPOINT"),
		MCPHint:       getenv("HAL_MCP_HINT"),
		Challenge: Challenge{
			ID: getenv("HAL_CHALLENGE_ID"), Slug: getenv("HAL_CHALLENGE_SLUG"),
			Name: getenv("HAL_CHALLENGE_NAME"), Category: getenv("HAL_CHALLENGE_CATEGORY"),
			Description: getenv("HAL_CHALLENGE_DESCRIPTION"),
		},
		Targets:     parseTargets(getenv),
		ModelLadder: parseLadder(getenv),
		MaxSteps:    40,
	}
	// MAX_STEPS is env-tunable; default 40, overridden when MAX_STEPS is a positive integer
	if v := getenv("MAX_STEPS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			c.MaxSteps = n
		}
	}
	if c.OpenAIBaseURL == "" {
		return c, fmt.Errorf("config: OPENAI_BASE_URL is required")
	}
	// MCP_ENDPOINT is optional: the critical path submits via the sidecar,
	// not MCP. When present it is still populated on Config for callers that
	// want it (e.g. an MCP-based fallback), but its absence is not an error.
	return c, nil
}

func parseLadder(getenv func(string) string) []string {
	src := getenv("MODEL_LADDER")
	if src == "" {
		if m := getenv("MODEL"); m != "" {
			src = m
		} else if m := getenv("HAL_AGENT_MODEL"); m != "" {
			src = m
		} else {
			src = defaultLadder
		}
	}
	var out []string
	for _, m := range strings.Split(src, ",") {
		if m = strings.TrimSpace(m); m != "" {
			out = append(out, m)
		}
	}
	return out
}

func parseTargets(getenv func(string) string) Targets {
	t := Targets{}
	if ip := getenv("HAL_TARGET_IP"); ip != "" {
		t[""] = Target{IP: ip, Port: getenv("HAL_TARGET_PORT")}
	}
	// Namespaced multi-target discovery is not implemented; only the default
	// HAL_TARGET_IP/HAL_TARGET_PORT pair above is parsed.
	return t
}

// ParseTargets builds the target map from the full process environment,
// covering both the default target (HAL_TARGET_IP / HAL_TARGET_PORT, keyed "")
// and namespaced multi-target challenges (HAL_TARGET_<NAME>_IP /
// HAL_TARGET_<NAME>_PORT, keyed by the lower-cased <NAME>). Pass os.Environ().
func ParseTargets(environ []string) Targets {
	t := Targets{}
	nameRe := regexp.MustCompile(`^HAL_TARGET_(.+)_(IP|PORT)$`)
	ensure := func(k string) Target { return t[k] }
	for _, kv := range environ {
		i := strings.IndexByte(kv, '=')
		if i < 0 {
			continue
		}
		key, val := kv[:i], kv[i+1:]
		switch key {
		case "HAL_TARGET_IP":
			d := ensure(""); d.IP = val; t[""] = d
			continue
		case "HAL_TARGET_PORT":
			d := ensure(""); d.Port = val; t[""] = d
			continue
		}
		m := nameRe.FindStringSubmatch(key)
		if m == nil {
			continue
		}
		name := strings.ToLower(m[1])
		tg := t[name]
		if m[2] == "IP" {
			tg.IP = val
		} else {
			tg.Port = val
		}
		t[name] = tg
	}
	return t
}
