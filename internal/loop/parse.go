package loop

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/Areontar/flatline/internal/model"
)

type ActionKind int

const (
	ActionNone ActionKind = iota
	ActionShell
	ActionSubmit
)

type Action struct {
	Kind       ActionKind
	Command    string
	Flag       string
	ToolCallID string
}

var (
	reAction = regexp.MustCompile(`(?im)^\s*Action\s*:\s*(.+?)\s*$`)
	reInput  = regexp.MustCompile(`(?is)Action\s*Input\s*:\s*(.+)$`)
)

func ParseAction(resp model.Response) (Action, bool) {
	// 1) Native tool calls.
	for _, tc := range resp.ToolCalls {
		switch tc.Name {
		case "run_shell":
			var a struct{ Command string `json:"command"` }
			_ = json.Unmarshal([]byte(tc.Arguments), &a)
			if a.Command != "" {
				return Action{Kind: ActionShell, Command: a.Command, ToolCallID: tc.ID}, true
			}
		case "submit_flag":
			var a struct{ Flag string `json:"flag"` }
			_ = json.Unmarshal([]byte(tc.Arguments), &a)
			if a.Flag != "" {
				return Action{Kind: ActionSubmit, Flag: a.Flag, ToolCallID: tc.ID}, true
			}
		}
	}
	// 2) Plain-text ReAct fallback.
	am := reAction.FindStringSubmatch(resp.Content)
	im := reInput.FindStringSubmatch(resp.Content)
	if am == nil || im == nil {
		return Action{}, false
	}
	verb := strings.ToLower(strings.TrimSpace(am[1]))
	input := strings.TrimSpace(im[1])
	switch {
	case strings.HasPrefix(verb, "run_shell"), strings.HasPrefix(verb, "shell"):
		return Action{Kind: ActionShell, Command: input}, true
	case strings.HasPrefix(verb, "submit_flag"), strings.HasPrefix(verb, "submit"):
		return Action{Kind: ActionSubmit, Flag: firstLine(input)}, true
	}
	return Action{}, false
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}
