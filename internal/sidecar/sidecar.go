// sidecar.go
package sidecar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"

	"github.com/Areontar/flatline/internal/mcp"
)

// Client talks to the HALCTF NGINX sidecar's plain-HTTP endpoints. This is the
// primary flag-submit and graceful-exit path; it needs no MCP session.
type Client struct {
	base      string
	challenge string
	http      *http.Client
}

// New builds a sidecar client. baseURL defaults to http://127.0.0.1:9000 when empty.
func New(baseURL, challengeID string) *Client {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:9000"
	}
	return &Client{base: baseURL, challenge: challengeID, http: &http.Client{Timeout: 30 * time.Second}}
}

// SubmitFlag posts a candidate flag to the sidecar and satisfies loop.Submitter.
func (c *Client) SubmitFlag(ctx context.Context, flag string) (mcp.SubmitResult, error) {
	body, _ := json.Marshal(map[string]any{"challenge_id": c.challenge, "flag": flag})
	raw, err := c.post(ctx, "/submit", body)
	if err != nil {
		return mcp.SubmitResult{}, err
	}
	var r struct {
		Correct       bool   `json:"correct"`
		AlreadySolved bool   `json:"already_solved"`
		AttemptsLeft  int    `json:"attempts_left"`
		Message       string `json:"message"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return mcp.SubmitResult{}, fmt.Errorf("sidecar submit: decode response: %w", err)
	}
	return mcp.SubmitResult(r), nil
}

// Done tells the sidecar the agent is finished, freeing the queue slot immediately.
func (c *Client) Done(ctx context.Context) error {
	_, err := c.post(ctx, "/done", []byte("{}"))
	return err
}

func (c *Client) post(ctx context.Context, path string, body []byte) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			d := time.Duration(1<<attempt)*200*time.Millisecond + time.Duration(rand.Intn(200))*time.Millisecond
			select {
			case <-time.After(d):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		req, _ := http.NewRequestWithContext(ctx, "POST", c.base+path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("sidecar %s: status %d", path, resp.StatusCode)
			continue
		}
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("sidecar %s: status %d: %s", path, resp.StatusCode, string(b))
		}
		return b, nil
	}
	return nil, fmt.Errorf("sidecar %s: %w", path, lastErr)
}
