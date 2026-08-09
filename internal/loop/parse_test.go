package loop

import (
	"testing"

	"github.com/Areontar/flatline/internal/model"
)

func TestParseNativeToolCall(t *testing.T) {
	a, ok := ParseAction(model.Response{ToolCalls: []model.ToolCall{
		{ID: "c1", Name: "run_shell", Arguments: `{"command":"nmap -p- 10.0.0.5"}`}}})
	if !ok || a.Kind != ActionShell || a.Command != "nmap -p- 10.0.0.5" || a.ToolCallID != "c1" {
		t.Fatalf("%+v ok=%v", a, ok)
	}
}

func TestParsePlainTextReAct(t *testing.T) {
	content := "Thought: I should scan\nAction: run_shell\nAction Input: curl http://x"
	a, ok := ParseAction(model.Response{Content: content})
	if !ok || a.Kind != ActionShell || a.Command != "curl http://x" {
		t.Fatalf("%+v ok=%v", a, ok)
	}
}

func TestParseSubmitPlainText(t *testing.T) {
	a, ok := ParseAction(model.Response{Content: "Action: submit_flag\nAction Input: flag{abc}\nextra"})
	if !ok || a.Kind != ActionSubmit || a.Flag != "flag{abc}" {
		t.Fatalf("%+v ok=%v", a, ok)
	}
}

func TestParseMalformed(t *testing.T) {
	if _, ok := ParseAction(model.Response{Content: "I don't know what to do"}); ok {
		t.Fatal("expected unparseable")
	}
}

func TestParseEmptyToolCallFallsThrough(t *testing.T) {
	resp := model.Response{
		ToolCalls: []model.ToolCall{{ID: "c1", Name: "run_shell", Arguments: `{"command":""}`}},
		Content:   "Thought: fallback\nAction: run_shell\nAction Input: ls -la",
	}
	a, ok := ParseAction(resp)
	if !ok || a.Kind != ActionShell || a.Command != "ls -la" {
		t.Fatalf("expected fallback to plain-text shell action, got %+v ok=%v", a, ok)
	}
	if a.ToolCallID != "" {
		t.Fatalf("fallback action should have no ToolCallID, got %q", a.ToolCallID)
	}
}
