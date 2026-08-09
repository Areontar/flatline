package sidecar

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSubmitFlagPostsAndParses(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Write([]byte(`{"correct":true,"attempts_left":3}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "chal-1")
	res, err := c.SubmitFlag(context.Background(), "flag{x}")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Correct || res.AttemptsLeft != 3 {
		t.Fatalf("%+v", res)
	}
	if gotPath != "/submit" || gotBody["flag"] != "flag{x}" || gotBody["challenge_id"] != "chal-1" {
		t.Fatalf("path=%s body=%v", gotPath, gotBody)
	}
}

func TestDonePostsToDone(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(200)
	}))
	defer srv.Close()

	if err := New(srv.URL, "c").Done(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/done" {
		t.Fatalf("path=%s", gotPath)
	}
}

func TestSubmitFlagRetriesOn500(t *testing.T) {
	var n int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		if n < 2 {
			w.WriteHeader(500)
			return
		}
		w.Write([]byte(`{"correct":false}`))
	}))
	defer srv.Close()
	if _, err := New(srv.URL, "c").SubmitFlag(context.Background(), "f"); err != nil {
		t.Fatal(err)
	}
	if n < 2 {
		t.Fatalf("expected retry, n=%d", n)
	}
}
