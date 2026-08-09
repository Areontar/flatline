package mcp

import (
	"context"
	"encoding/json"
	"testing"
)

type stubTransport struct {
	lastTool string
	lastArgs map[string]any
	reply    json.RawMessage
}

func (s *stubTransport) Call(_ context.Context, tool string, args map[string]any) (json.RawMessage, error) {
	s.lastTool, s.lastArgs = tool, args
	return s.reply, nil
}

func TestSubmitFlagParses(t *testing.T) {
	st := &stubTransport{reply: json.RawMessage(`{"correct":true,"attempts_left":2}`)}
	c := New(st, "chal-1")
	r, err := c.SubmitFlag(context.Background(), "flag{x}")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Correct || r.AttemptsLeft != 2 {
		t.Fatalf("%+v", r)
	}
	if st.lastTool != "submit_flag" || st.lastArgs["flag"] != "flag{x}" || st.lastArgs["challenge_id"] != "chal-1" {
		t.Fatalf("args: %v", st.lastArgs)
	}
}

func TestGracefulExit(t *testing.T) {
	st := &stubTransport{reply: json.RawMessage(`{}`)}
	if err := New(st, "c").GracefulExit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if st.lastTool != "graceful_exit" {
		t.Fatalf("tool=%s", st.lastTool)
	}
}

func TestSubmitFlagMalformedJSON(t *testing.T) {
	st := &stubTransport{reply: json.RawMessage("not json")}
	c := New(st, "chal-1")
	_, err := c.SubmitFlag(context.Background(), "flag{x}")
	if err == nil {
		t.Fatal("expected error on malformed JSON, got nil")
	}
}
