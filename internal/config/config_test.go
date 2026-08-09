package config

import "testing"

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadRequiresEndpoints(t *testing.T) {
	if _, err := Load(env(map[string]string{})); err == nil {
		t.Fatal("expected error when OPENAI_BASE_URL missing")
	}
}

func TestLoadParsesLadderAndTarget(t *testing.T) {
	c, err := Load(env(map[string]string{
		"OPENAI_BASE_URL": "http://x/v1/",
		"MCP_ENDPOINT":    "http://y",
		"MODEL_LADDER":    "a, b ,c",
		"HAL_TARGET_IP":   "10.0.0.5",
		"HAL_TARGET_PORT": "8080",
		"HAL_USER_ID":     "u1",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.OpenAIBaseURL != "http://x/v1" {
		t.Fatalf("trailing slash not trimmed: %q", c.OpenAIBaseURL)
	}
	if got := c.ModelLadder; len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Fatalf("ladder parse: %v", got)
	}
	if c.Targets[""].IP != "10.0.0.5" || c.Targets[""].Port != "8080" {
		t.Fatalf("target parse: %+v", c.Targets)
	}
}

func TestLoadDefaultLadder(t *testing.T) {
	c, _ := Load(env(map[string]string{"OPENAI_BASE_URL": "x", "MCP_ENDPOINT": "y"}))
	if len(c.ModelLadder) != 2 || c.ModelLadder[0] != "qwen3.6-35b-a3b" {
		t.Fatalf("default ladder: %v", c.ModelLadder)
	}
}

// TestLoadMCPEndpointOptional verifies that a missing MCP_ENDPOINT does not
// fail config loading: the critical submit/graceful-done path goes through
// the sidecar, not MCP, so MCP_ENDPOINT must be optional.
func TestLoadMCPEndpointOptional(t *testing.T) {
	c, err := Load(env(map[string]string{
		"OPENAI_BASE_URL": "http://x/v1",
	}))
	if err != nil {
		t.Fatalf("expected no error with MCP_ENDPOINT empty, got: %v", err)
	}
	if c.MCPEndpoint != "" {
		t.Fatalf("expected empty MCPEndpoint, got %q", c.MCPEndpoint)
	}
}

func TestLoadUsesHalAgentModel(t *testing.T) {
	c, err := Load(env(map[string]string{
		"OPENAI_BASE_URL": "http://x/v1",
		"MCP_ENDPOINT":    "http://y",
		"HAL_AGENT_MODEL": "qwen3.6-35b-a3b",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.ModelLadder) != 1 || c.ModelLadder[0] != "qwen3.6-35b-a3b" {
		t.Fatalf("expected [qwen3.6-35b-a3b], got %v", c.ModelLadder)
	}
}

func TestLoadModelPrecedence(t *testing.T) {
	c, err := Load(env(map[string]string{
		"OPENAI_BASE_URL": "http://x/v1",
		"MCP_ENDPOINT":    "http://y",
		"MODEL":           "foo",
		"HAL_AGENT_MODEL": "bar",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.ModelLadder) != 1 || c.ModelLadder[0] != "foo" {
		t.Fatalf("expected MODEL to win over HAL_AGENT_MODEL: got %v", c.ModelLadder)
	}
}

func TestLoadMaxStepsEnv(t *testing.T) {
	// Test with MAX_STEPS set to 75
	c, err := Load(env(map[string]string{
		"OPENAI_BASE_URL": "http://x/v1",
		"MCP_ENDPOINT":    "http://y",
		"MAX_STEPS":       "75",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.MaxSteps != 75 {
		t.Fatalf("expected MaxSteps=75, got %d", c.MaxSteps)
	}

	// Test with MAX_STEPS unset (should default to 40)
	c, err = Load(env(map[string]string{
		"OPENAI_BASE_URL": "http://x/v1",
		"MCP_ENDPOINT":    "http://y",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.MaxSteps != 40 {
		t.Fatalf("expected MaxSteps=40 (default), got %d", c.MaxSteps)
	}
}

func TestParseTargetsNamespaced(t *testing.T) {
	environ := []string{
		"HAL_TARGET_IP=1.2.3.4",
		"HAL_TARGET_PORT=80",
		"HAL_TARGET_WEB_IP=5.6.7.8",
		"HAL_TARGET_WEB_PORT=8080",
		"HAL_TARGET_DB_IP=9.9.9.9",
		"OTHER=x",
	}
	targets := ParseTargets(environ)

	// Check default target
	if targets[""].IP != "1.2.3.4" || targets[""].Port != "80" {
		t.Fatalf("expected default target {IP:1.2.3.4, Port:80}, got %+v", targets[""])
	}

	// Check web target
	if targets["web"].IP != "5.6.7.8" || targets["web"].Port != "8080" {
		t.Fatalf("expected web target {IP:5.6.7.8, Port:8080}, got %+v", targets["web"])
	}

	// Check db target (port is empty)
	if targets["db"].IP != "9.9.9.9" || targets["db"].Port != "" {
		t.Fatalf("expected db target {IP:9.9.9.9, Port:}, got %+v", targets["db"])
	}

	// Check no spurious keys
	if len(targets) != 3 {
		t.Fatalf("expected 3 targets, got %d: %v", len(targets), targets)
	}
}
