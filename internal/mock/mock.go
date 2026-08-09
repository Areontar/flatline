package mock

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
)

// NewSidecarServer emulates the HALCTF NGINX sidecar's flag endpoints:
// POST /submit returns {"correct": flag==planted}; POST /done returns {}.
func NewSidecarServer(planted string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/submit":
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Flag string `json:"flag"`
			}
			_ = json.Unmarshal(body, &req)
			json.NewEncoder(w).Encode(map[string]any{"correct": req.Flag == planted})
		case "/done":
			w.Write([]byte(`{}`))
		default:
			w.WriteHeader(404)
		}
	}))
}

// NewTarget serves the planted flag at /secret (a stand-in challenge target).
func NewTarget(flag string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/secret") {
			w.Write([]byte("the flag is " + flag))
			return
		}
		w.WriteHeader(404)
	}))
}
