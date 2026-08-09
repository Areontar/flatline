package mcp

import (
	"context"
	"encoding/json"
	"fmt"
)

type Client struct {
	t         Transport
	challenge string // HAL_CHALLENGE_ID, bound at construction
}

func New(t Transport, challengeID string) *Client { return &Client{t: t, challenge: challengeID} }

type SubmitResult struct {
	Correct       bool
	AlreadySolved bool
	AttemptsLeft  int
	Message       string
}

func (c *Client) SubmitFlag(ctx context.Context, flag string) (SubmitResult, error) {
	raw, err := c.t.Call(ctx, "submit_flag", map[string]any{"challenge_id": c.challenge, "flag": flag})
	if err != nil {
		return SubmitResult{}, err
	}
	var r struct {
		Correct       bool   `json:"correct"`
		AlreadySolved bool   `json:"already_solved"`
		AttemptsLeft  int    `json:"attempts_left"`
		Message       string `json:"message"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return SubmitResult{}, fmt.Errorf("unmarshal submit_flag response: %w", err)
	}
	return SubmitResult(r), nil
}

func (c *Client) GetChallenge(ctx context.Context) (json.RawMessage, error) {
	return c.t.Call(ctx, "get_challenge", map[string]any{"challenge_id": c.challenge})
}
func (c *Client) GetChallengeStatus(ctx context.Context) (json.RawMessage, error) {
	return c.t.Call(ctx, "get_challenge_status", map[string]any{"challenge_id": c.challenge})
}
func (c *Client) RequestHint(ctx context.Context, idx int) (json.RawMessage, error) {
	return c.t.Call(ctx, "request_hint", map[string]any{"challenge_id": c.challenge, "hint_index": idx})
}
func (c *Client) GracefulExit(ctx context.Context) error {
	_, err := c.t.Call(ctx, "graceful_exit", map[string]any{})
	return err
}
