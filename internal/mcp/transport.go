package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"
)

type Transport interface {
	Call(ctx context.Context, tool string, args map[string]any) (json.RawMessage, error)
}

// HTTPTransport implements MCP tools/call over JSON-RPC 2.0 (assumed; confirm
// against HAL_MCP_HINT and swap the body shape here if the platform differs).
type HTTPTransport struct {
	Endpoint string
	HTTP     *http.Client
}

func NewHTTPTransport(endpoint string) *HTTPTransport {
	return &HTTPTransport{Endpoint: endpoint, HTTP: &http.Client{Timeout: 30 * time.Second}}
}

func (t *HTTPTransport) Call(ctx context.Context, tool string, args map[string]any) (json.RawMessage, error) {
	reqBody, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": args},
	})
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			backoff(ctx, attempt)
		}
		req, _ := http.NewRequestWithContext(ctx, "POST", t.Endpoint, bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		resp, err := t.HTTP.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("mcp %s: status %d", tool, resp.StatusCode)
			continue
		}
		var env struct {
			Result json.RawMessage `json:"result"`
			Error  *struct{ Message string } `json:"error"`
		}
		if err := json.Unmarshal(body, &env); err != nil {
			// Fall back to treating the whole body as the result (REST-style).
			return body, nil
		}
		if env.Error != nil {
			return nil, fmt.Errorf("mcp %s: %s", tool, env.Error.Message)
		}
		return env.Result, nil
	}
	return nil, fmt.Errorf("mcp %s: %w", tool, lastErr)
}

func backoff(ctx context.Context, attempt int) {
	d := time.Duration(1<<attempt)*200*time.Millisecond + time.Duration(rand.Intn(200))*time.Millisecond
	select {
	case <-time.After(d):
	case <-ctx.Done():
	}
}
