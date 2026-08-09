package model

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

type OpenAIClient struct {
	baseURL string
	model   string
	http    *http.Client
}

func NewOpenAI(baseURL, modelName string) *OpenAIClient {
	return &OpenAIClient{baseURL: baseURL, model: modelName, http: &http.Client{Timeout: 120 * time.Second}}
}
func (c *OpenAIClient) Model() string { return c.model }

func (c *OpenAIClient) Complete(ctx context.Context, msgs []Message, tools []Tool) (Response, error) {
	body := map[string]any{"model": c.model, "messages": toWireMessages(msgs)}
	if len(tools) > 0 {
		body["tools"] = toWireTools(tools)
	}
	raw, _ := json.Marshal(body)
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			d := time.Duration(1<<attempt)*300*time.Millisecond + time.Duration(rand.Intn(300))*time.Millisecond
			select {
			case <-time.After(d):
			case <-ctx.Done():
				return Response{}, ctx.Err()
			}
		}
		req, _ := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("model status %d", resp.StatusCode)
			continue
		}
		if resp.StatusCode >= 400 {
			return Response{}, fmt.Errorf("model status %d: %s", resp.StatusCode, string(b))
		}
		return parseResponse(b)
	}
	return Response{}, fmt.Errorf("model complete: %w", lastErr)
}

func toWireMessages(msgs []Message) []map[string]any {
	out := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		w := map[string]any{"role": m.Role, "content": m.Content}
		if m.ToolCallID != "" {
			w["tool_call_id"] = m.ToolCallID
		}
		if len(m.ToolCalls) > 0 {
			var tc []map[string]any
			for _, t := range m.ToolCalls {
				tc = append(tc, map[string]any{"id": t.ID, "type": "function",
					"function": map[string]any{"name": t.Name, "arguments": t.Arguments}})
			}
			w["tool_calls"] = tc
		}
		out = append(out, w)
	}
	return out
}

func toWireTools(tools []Tool) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		out = append(out, map[string]any{"type": "function",
			"function": map[string]any{"name": t.Name, "description": t.Description, "parameters": t.Parameters}})
	}
	return out
}

func parseResponse(b []byte) (Response, error) {
	var wire struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(b, &wire); err != nil {
		return Response{}, fmt.Errorf("model decode: %w", err)
	}
	if len(wire.Choices) == 0 {
		return Response{}, fmt.Errorf("model: empty choices")
	}
	r := Response{
		Content:          wire.Choices[0].Message.Content,
		PromptTokens:     wire.Usage.PromptTokens,
		CompletionTokens: wire.Usage.CompletionTokens,
	}
	for _, tc := range wire.Choices[0].Message.ToolCalls {
		r.ToolCalls = append(r.ToolCalls, ToolCall{ID: tc.ID, Name: tc.Function.Name, Arguments: tc.Function.Arguments})
	}
	return r, nil
}
