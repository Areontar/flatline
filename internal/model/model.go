package model

import "context"

type Message struct {
	Role       string     // "system" | "user" | "assistant" | "tool"
	Content    string
	ToolCalls  []ToolCall // assistant tool calls
	ToolCallID string     // for role "tool"
	Name       string
}
type ToolCall struct {
	ID        string
	Name      string
	Arguments string // raw JSON args string
}
type Tool struct {
	Name        string
	Description string
	Parameters  map[string]any // JSON schema
}
type Response struct {
	Content          string
	ToolCalls        []ToolCall
	PromptTokens     int
	CompletionTokens int
}
type Chat interface {
	Complete(ctx context.Context, msgs []Message, tools []Tool) (Response, error)
	Model() string
}
