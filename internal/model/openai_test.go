package model

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompleteParsesToolCallAndUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"choices":[{"message":{"content":"hi","tool_calls":[{"id":"c1","function":{"name":"run_shell","arguments":"{\"command\":\"ls\"}"}}]}}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`))
	}))
	defer srv.Close()
	c := NewOpenAI(srv.URL, "test-model")
	resp, err := c.Complete(context.Background(), []Message{{Role: "user", Content: "go"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "hi" || len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Name != "run_shell" {
		t.Fatalf("%+v", resp)
	}
	if resp.PromptTokens != 10 || resp.CompletionTokens != 5 {
		t.Fatalf("usage: %+v", resp)
	}
}

func TestCompleteRetriesOn500(t *testing.T) {
	var n int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		if n < 2 {
			w.WriteHeader(500)
			return
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}],"usage":{}}`))
	}))
	defer srv.Close()
	c := NewOpenAI(srv.URL, "m")
	resp, err := c.Complete(context.Background(), []Message{{Role: "user", Content: "x"}}, nil)
	if err != nil || resp.Content != "ok" || n < 2 {
		t.Fatalf("retry failed: n=%d resp=%+v err=%v", n, resp, err)
	}
}
