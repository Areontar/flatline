package loop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Areontar/flatline/internal/mcp"
	"github.com/Areontar/flatline/internal/model"
	"github.com/Areontar/flatline/internal/shell"
)

type scriptedChat struct {
	steps    []model.Response
	i        int
	lastMsgs []model.Message
}

func (s *scriptedChat) Model() string { return "scripted" }
func (s *scriptedChat) Complete(_ context.Context, msgs []model.Message, _ []model.Tool) (model.Response, error) {
	s.lastMsgs = msgs
	r := s.steps[s.i]
	if s.i < len(s.steps)-1 {
		s.i++
	}
	return r, nil
}

type fakeSub struct{ correctOn string }

func (f fakeSub) SubmitFlag(_ context.Context, flag string) (mcp.SubmitResult, error) {
	return mcp.SubmitResult{Correct: flag == f.correctOn}, nil
}

func ladderOf(c model.Chat) *model.Ladder {
	return model.NewLadder([]string{"scripted"}, func(string) model.Chat { return c }, 0)
}

func TestLoopSolves(t *testing.T) {
	chat := &scriptedChat{steps: []model.Response{
		{Content: "Thought: scan\nAction: run_shell\nAction Input: cat flag.txt"},
		{Content: "Thought: found it\nAction: submit_flag\nAction Input: flag{win}"},
	}}
	sh := func(_ context.Context, cmd string) shell.Result { return shell.Result{Stdout: "flag{win}"} }
	e := NewEngine(ladderOf(chat), fakeSub{correctOn: "flag{win}"}, sh, Spec{FlagRegex: `flag\{.+\}`}, 10, nil)
	solved, m := e.Run(context.Background(), "solve it")
	if !solved || m != "scripted" {
		t.Fatalf("solved=%v model=%s", solved, m)
	}
}

func TestLoopRejectsRepeatedShell(t *testing.T) {
	chat := &scriptedChat{steps: []model.Response{
		{Content: "Action: run_shell\nAction Input: ls"}, // repeated forever
	}}
	var runs int
	sh := func(_ context.Context, cmd string) shell.Result { runs++; return shell.Result{Stdout: "x"} }
	e := NewEngine(ladderOf(chat), fakeSub{}, sh, Spec{FlagRegex: `flag\{.+\}`}, 6, nil)
	e.Run(context.Background(), "go")
	if runs > 1 {
		t.Fatalf("repeated command ran %d times, expected 1", runs)
	}
}

func TestCompactKeepsSystemAndRecent(t *testing.T) {
	const keepRecent = 12
	msgs := []model.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "first-user"},
	}
	for i := 0; i < 20; i++ {
		msgs = append(msgs, model.Message{Role: "user", Content: fmt.Sprintf("filler-%d", i)})
	}

	got := compact(msgs, keepRecent)

	if len(got) != keepRecent+2 {
		t.Fatalf("len(got)=%d, want %d", len(got), keepRecent+2)
	}
	if got[0].Content != "sys" {
		t.Fatalf("got[0]=%q, want system message preserved", got[0].Content)
	}
	if got[1].Content != "first-user" {
		t.Fatalf("got[1]=%q, want first user message preserved", got[1].Content)
	}
	wantTail := msgs[len(msgs)-keepRecent:]
	for i, m := range wantTail {
		if got[2+i].Content != m.Content {
			t.Fatalf("got[%d]=%q, want %q", 2+i, got[2+i].Content, m.Content)
		}
	}

	short := []model.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "first-user"},
		{Role: "user", Content: "only-other"},
	}
	gotShort := compact(short, keepRecent)
	if len(gotShort) != len(short) {
		t.Fatalf("short slice len=%d, want unchanged %d", len(gotShort), len(short))
	}
	for i := range short {
		if gotShort[i].Content != short[i].Content {
			t.Fatalf("short slice mutated at %d: got %q want %q", i, gotShort[i].Content, short[i].Content)
		}
	}
}

// TestCompactDoesNotOrphanToolMessage builds a history long enough that a
// naive fixed-length-12 tail would start on a role:"tool" message (which,
// per the OpenAI wire contract, must always be immediately preceded by the
// assistant message carrying the matching tool_call). compact must walk the
// cut point backward until the tail starts on a non-tool message.
func TestCompactDoesNotOrphanToolMessage(t *testing.T) {
	const keepRecent = 12
	msgs := []model.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "first-user"},
	}
	// indices 2..24 (23 messages), alternating assistant(with ToolCalls)/tool,
	// starting with assistant at index 2. With len(msgs)==25 and keepRecent==12,
	// cut = 25-12 = 13, which lands on a "tool" message (odd index >= 3 in this
	// pattern) under the naive (unfixed) algorithm.
	for i := 0; i < 23; i++ {
		if i%2 == 0 {
			msgs = append(msgs, model.Message{
				Role:      "assistant",
				ToolCalls: []model.ToolCall{{ID: fmt.Sprintf("tc-%d", i), Name: "run_shell"}},
			})
		} else {
			msgs = append(msgs, model.Message{Role: "tool", ToolCallID: fmt.Sprintf("tc-%d", i-1), Content: "out"})
		}
	}
	if len(msgs) != 25 {
		t.Fatalf("test setup: len(msgs)=%d, want 25", len(msgs))
	}

	got := compact(msgs, keepRecent)

	for i := 2; i < len(got); i++ {
		if got[i].Role == "tool" {
			prev := got[i-1]
			if prev.Role != "assistant" || len(prev.ToolCalls) == 0 {
				t.Fatalf("orphaned tool message at out[%d]: predecessor out[%d]=%+v is not an assistant carrying ToolCalls", i, i-1, prev)
			}
		}
	}
}

// TestLoopEscalatesOnRepeatedWrongGuesses verifies that repeated incorrect
// flag submissions count as stalls and eventually trigger ladder escalation
// (previously wrongGuesses was tracked but never fed into the stall counter,
// so maybeEscalate's stalls>=3 threshold was never reached via this path).
func TestLoopEscalatesOnRepeatedWrongGuesses(t *testing.T) {
	chat := &scriptedChat{steps: []model.Response{
		{Content: "Thought: guess\nAction: submit_flag\nAction Input: flag{wrong}"},
	}}
	ladder := model.NewLadder([]string{"a", "b"}, func(string) model.Chat { return chat }, 0)
	sh := func(_ context.Context, cmd string) shell.Result { return shell.Result{} }
	e := NewEngine(ladder, fakeSub{}, sh, Spec{FlagRegex: `flag\{.+\}`}, 10, nil)
	e.Run(context.Background(), "go")
	if ladder.Model() != "b" {
		t.Fatalf("ladder.Model()=%s, want escalated to b after repeated wrong guesses", ladder.Model())
	}
}

// TestLoopAnswersNativeToolCallRepeat verifies that when the model repeats
// the same native tool call, the loop's repeat-detected nudge still answers
// the pending tool_call with a role:"tool" message (per the OpenAI wire
// contract), rather than a bare role:"user" message that would leave the
// tool_call unanswered.
func TestLoopAnswersNativeToolCallRepeat(t *testing.T) {
	chat := &scriptedChat{steps: []model.Response{
		{ToolCalls: []model.ToolCall{{ID: "c1", Name: "run_shell", Arguments: `{"command":"ls"}`}}},
	}}
	sh := func(_ context.Context, cmd string) shell.Result { return shell.Result{Stdout: "x"} }
	e := NewEngine(ladderOf(chat), fakeSub{}, sh, Spec{FlagRegex: `flag\{.+\}`}, 6, nil)
	e.Run(context.Background(), "go")

	msgs := chat.lastMsgs
	if len(msgs) == 0 {
		t.Fatal("scriptedChat never captured any messages")
	}
	for i, m := range msgs {
		if m.Role == "tool" {
			if i == 0 || msgs[i-1].Role != "assistant" || len(msgs[i-1].ToolCalls) == 0 {
				t.Fatalf("tool message at %d not immediately preceded by an assistant carrying ToolCalls: prev=%+v", i, msgs[i-1])
			}
		}
		if m.Role == "assistant" && len(m.ToolCalls) > 0 && i+1 < len(msgs) && msgs[i+1].Role == "user" {
			t.Fatalf("assistant-with-ToolCalls at %d immediately followed by role:\"user\" (unanswered tool_call)", i)
		}
	}
}

// TestLoopAnswersAllToolCalls verifies that when the model emits two native
// tool_calls in a single turn, both get a matching role:"tool" result before
// the next request is sent -- only the first (c1) is actually executed, but
// the second (c2) must still be answered with a stub result, or the next
// request would 400 against the OpenAI wire contract.
func TestLoopAnswersAllToolCalls(t *testing.T) {
	chat := &scriptedChat{steps: []model.Response{
		{ToolCalls: []model.ToolCall{
			{ID: "c1", Name: "run_shell", Arguments: `{"command":"ls"}`},
			{ID: "c2", Name: "run_shell", Arguments: `{"command":"whoami"}`},
		}},
		{Content: "Thought: done\nAction: submit_flag\nAction Input: flag{nope}"},
	}}
	sh := func(_ context.Context, cmd string) shell.Result { return shell.Result{Stdout: "x"} }
	e := NewEngine(ladderOf(chat), fakeSub{}, sh, Spec{FlagRegex: `flag\{.+\}`}, 6, nil)
	e.Run(context.Background(), "go")

	// lastMsgs was captured on the second Complete() call, i.e. the msgs the
	// fake was invoked with on turn 2 -- must contain tool results for both ids.
	msgs := chat.lastMsgs
	var sawC1, sawC2 bool
	for _, m := range msgs {
		if m.Role == "tool" && m.ToolCallID == "c1" {
			sawC1 = true
		}
		if m.Role == "tool" && m.ToolCallID == "c2" {
			sawC2 = true
		}
	}
	if !sawC1 || !sawC2 {
		t.Fatalf("expected tool results for both c1 and c2, sawC1=%v sawC2=%v, msgs=%+v", sawC1, sawC2, msgs)
	}
}

// TestLoopSkipsRepeatedWrongFlag verifies that once a flag has come back
// incorrect from the platform, an identical resubmission is short-circuited
// locally instead of burning another SubmitFlag call against the platform.
func TestLoopSkipsRepeatedWrongFlag(t *testing.T) {
	chat := &scriptedChat{steps: []model.Response{
		{Content: "Thought: guess\nAction: submit_flag\nAction Input: flag{wrong}"},
	}}
	sub := &countingSub{}
	sh := func(_ context.Context, cmd string) shell.Result { return shell.Result{} }
	e := NewEngine(ladderOf(chat), sub, sh, Spec{FlagRegex: `flag\{.+\}`}, 10, nil)
	e.Run(context.Background(), "go")
	if sub.calls["flag{wrong}"] > 1 {
		t.Fatalf("SubmitFlag called %d times for the same wrong flag, want at most 1", sub.calls["flag{wrong}"])
	}
}

// TestLoopSanitizesMalformedToolCall verifies that a tool_call the model
// emitted with truncated/malformed (non-JSON) Arguments -- e.g. a tool call
// cut off mid-string by a context-window overrun -- is sanitized to "{}"
// before being stored in the conversation history. Without this, the raw
// malformed string is re-sent verbatim on every subsequent request via
// toWireMessages, and some model backends respond HTTP 500 trying to
// re-parse it, poisoning the entire rest of the run.
func TestLoopSanitizesMalformedToolCall(t *testing.T) {
	chat := &scriptedChat{steps: []model.Response{
		{ToolCalls: []model.ToolCall{{ID: "c1", Name: "run_shell", Arguments: `{"command":"curl unterminated`}}},
	}}
	sh := func(_ context.Context, cmd string) shell.Result { return shell.Result{Stdout: "x"} }
	e := NewEngine(ladderOf(chat), fakeSub{}, sh, Spec{FlagRegex: `flag\{.+\}`}, 3, nil)
	e.Run(context.Background(), "go")

	msgs := chat.lastMsgs
	var found bool
	for _, m := range msgs {
		if m.Role != "assistant" {
			continue
		}
		for _, tc := range m.ToolCalls {
			if tc.ID != "c1" {
				continue
			}
			found = true
			if !json.Valid([]byte(tc.Arguments)) {
				t.Fatalf("stored tool_call c1 Arguments is not valid JSON (history poisoned): %q", tc.Arguments)
			}
		}
	}
	if !found {
		t.Fatal("assistant message carrying tool_call c1 not found in captured msgs")
	}
}

// erroringChat always fails Complete, simulating a wedged model backend
// (HTTP 500s or timeouts). Used by TestLoopGivesUpAfterConsecutiveModelErrors
// to verify the loop circuit-breaks instead of retrying for the whole step
// budget.
type erroringChat struct{ calls int }

func (e *erroringChat) Model() string { return "err" }
func (e *erroringChat) Complete(_ context.Context, _ []model.Message, _ []model.Tool) (model.Response, error) {
	e.calls++
	return model.Response{}, errors.New("backend down")
}

// TestLoopGivesUpAfterConsecutiveModelErrors verifies that when the model
// backend fails repeatedly, the loop gives up after maxConsecutiveModelErrors
// consecutive failures rather than retrying every step until the whole step
// budget is exhausted (which would hold the shared queue slot the entire
// time on a genuinely wedged backend).
func TestLoopGivesUpAfterConsecutiveModelErrors(t *testing.T) {
	ec := &erroringChat{}
	ladder := model.NewLadder([]string{"err"}, func(string) model.Chat { return ec }, 0)
	sh := func(_ context.Context, cmd string) shell.Result { return shell.Result{} }
	e := NewEngine(ladder, fakeSub{}, sh, Spec{FlagRegex: `flag\{.+\}`}, 40, nil)

	start := time.Now()
	solved, _ := e.Run(context.Background(), "go")
	elapsed := time.Since(start)

	if solved {
		t.Fatalf("solved=true, want false for an always-failing backend")
	}
	if ec.calls > maxConsecutiveModelErrors {
		t.Fatalf("Complete called %d times, want <= %d (breaker should trip instead of burning the whole step budget)", ec.calls, maxConsecutiveModelErrors)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("Run took %s, want fast (breaker should trip well before the 40-step budget)", elapsed)
	}
}

// TestLoopLoggingSignals verifies the dashboard log stream (e.log) carries
// actionable debugging signal: a step header, the shell command's exit
// status, the "possible unrecognized flag" warning for a flag-shaped string
// that does not match the active flag regex, a submit outcome line, and a
// cumulative token-usage line.
func TestLoopLoggingSignals(t *testing.T) {
	chat := &scriptedChat{steps: []model.Response{
		{Content: "Thought: scan\nAction: run_shell\nAction Input: cat flag.txt",
			PromptTokens: 10, CompletionTokens: 5},
		{Content: "Thought: found it\nAction: submit_flag\nAction Input: flag{win}",
			PromptTokens: 8, CompletionTokens: 3},
	}}
	sh := func(_ context.Context, cmd string) shell.Result {
		return shell.Result{Stdout: "banner\nweird{unrecognized_flag_here}\n", ExitCode: 0}
	}
	var logs []string
	logf := func(s string) { logs = append(logs, s) }
	e := NewEngine(ladderOf(chat), fakeSub{correctOn: "flag{win}"}, sh, Spec{FlagRegex: `flag\{.+\}`}, 10, logf)
	solved, _ := e.Run(context.Background(), "solve it")
	if !solved {
		t.Fatalf("solved=false, want true")
	}

	has := func(substr string) bool {
		for _, l := range logs {
			if strings.Contains(l, substr) {
				return true
			}
		}
		return false
	}
	if !has("--- step 1/") {
		t.Errorf("missing step header line; logs=%v", logs)
	}
	if !has("[exit 0]") {
		t.Errorf("missing exit status line; logs=%v", logs)
	}
	if !has("⚠ possible unrecognized flag") || !has("weird{unrecognized_flag_here}") {
		t.Errorf("missing unrecognized-flag warning for weird{unrecognized_flag_here}; logs=%v", logs)
	}
	if !has("submit ") {
		t.Errorf("missing submit outcome line; logs=%v", logs)
	}
	if !has("tokens:") {
		t.Errorf("missing tokens line; logs=%v", logs)
	}
}

// TestPossibleFlagDetectorIgnoresCSS verifies possibleFlagRe (used only for
// logging) rejects CSS/JS-shaped brace blocks (which contain ':', ';', or
// whitespace) while still matching genuine flag-shaped strings.
func TestPossibleFlagDetectorIgnoresCSS(t *testing.T) {
	if possibleFlagRe.MatchString("body{color:red;}") {
		t.Error("possibleFlagRe should not match body{color:red;} (CSS)")
	}
	if possibleFlagRe.MatchString(".nav{margin:0}") {
		t.Error("possibleFlagRe should not match .nav{margin:0} (CSS)")
	}
	if !possibleFlagRe.MatchString("hacman{abc123def}") {
		t.Error("possibleFlagRe should match hacman{abc123def}")
	}
}

type countingSub struct{ calls map[string]int }

func (s *countingSub) SubmitFlag(_ context.Context, flag string) (mcp.SubmitResult, error) {
	if s.calls == nil {
		s.calls = map[string]int{}
	}
	s.calls[flag]++
	return mcp.SubmitResult{Correct: false}, nil
}

// countingCorrectSub is like countingSub but reports Correct for one specific
// flag, so tests can assert both that SubmitFlag was actually invoked for a
// candidate AND that the run solved as a result.
type countingCorrectSub struct {
	correctOn string
	calls     map[string]int
}

func (s *countingCorrectSub) SubmitFlag(_ context.Context, flag string) (mcp.SubmitResult, error) {
	if s.calls == nil {
		s.calls = map[string]int{}
	}
	s.calls[flag]++
	return mcp.SubmitResult{Correct: flag == s.correctOn}, nil
}

// TestLoopAutoSubmitsFlagFromShellOutput verifies that when a run_shell
// observation contains a flag matching the specialist's FlagRegex, the loop
// submits it directly -- even though the model itself never calls
// submit_flag. This is a reliability short-circuit: small models often SEE a
// flag in command output but fail to submit it (or hallucinate a different
// one), so the harness must not depend solely on the model's own action.
func TestLoopAutoSubmitsFlagFromShellOutput(t *testing.T) {
	chat := &scriptedChat{steps: []model.Response{
		{Content: "Thought: look around\nAction: run_shell\nAction Input: cat secret.txt"},
		// The model never emits submit_flag -- if auto-submit didn't exist,
		// this run would never solve (steps repeat this run_shell forever
		// via s.steps[s.i] holding at the last entry, and the repeated-shell
		// guard would stall it out).
	}}
	sh := func(_ context.Context, cmd string) shell.Result {
		return shell.Result{Stdout: "some noise\nthe secret is flag{auto_win} enjoy\nmore noise"}
	}
	sub := &countingCorrectSub{correctOn: "flag{auto_win}"}
	e := NewEngine(ladderOf(chat), sub, sh, Spec{FlagRegex: `(?i)(flag|ctf|hal)\{[^}]{1,120}\}`}, 10, nil)

	solved, _ := e.Run(context.Background(), "go")

	if !solved {
		t.Fatalf("solved=false, want true (auto-submit should have captured flag{auto_win} from shell output)")
	}
	if sub.calls["flag{auto_win}"] < 1 {
		t.Fatalf("SubmitFlag was never called with flag{auto_win}; auto-submit did not fire")
	}
}

// stagnantChat always returns an empty, unparseable response (no Content, no
// ToolCalls), simulating a weak model emitting persistent garbage every
// turn. Used by TestLoopGivesUpOnStagnation to verify the stagnation guard
// trips instead of burning the whole step budget on a model that never
// produces a diverse -- or even parseable -- action.
type stagnantChat struct{ calls int }

func (s *stagnantChat) Model() string { return "stagnant" }
func (s *stagnantChat) Complete(_ context.Context, _ []model.Message, _ []model.Tool) (model.Response, error) {
	s.calls++
	return model.Response{}, nil
}

// TestLoopGivesUpOnStagnation verifies that when the model's recent action
// window collapses to too few distinct actions (here: unparseable every
// single turn, so the window is 100% "<unparseable>"), the loop gives up via
// the stagnation guard well before exhausting the step budget -- freeing the
// shared queue slot instead of burning all maxSteps on a model that is
// cycling dead-end/garbage output. This complements repeat-detection (exact
// repeats) and the model-error circuit breaker (backend failures), neither
// of which catches this case: the response is a (non-error) success from the
// backend's point of view, just useless.
func TestLoopGivesUpOnStagnation(t *testing.T) {
	sc := &stagnantChat{}
	ladder := model.NewLadder([]string{"stagnant"}, func(string) model.Chat { return sc }, 0)
	sh := func(_ context.Context, cmd string) shell.Result { return shell.Result{Stdout: "no flag here"} }
	const maxSteps = 40
	e := NewEngine(ladder, fakeSub{}, sh, Spec{FlagRegex: `flag\{.+\}`}, maxSteps, nil)

	solved, _ := e.Run(context.Background(), "go")

	if solved {
		t.Fatalf("solved=true, want false for a persistently unparseable model")
	}
	if sc.calls >= maxSteps {
		t.Fatalf("Complete called %d times, want well under maxSteps=%d (stagnation guard should trip early)", sc.calls, maxSteps)
	}
	// The window fills at call 15 (first evaluation, 1st warning), then trips
	// on call 16 (2nd warning >= stagnationMaxWarnings) since every wkey is
	// identical ("<unparseable>"), so distinct count is always 1.
	if sc.calls != stagnationWindow+stagnationMaxWarnings-1 {
		t.Fatalf("Complete called %d times, want exactly %d (window=%d, warnings=%d)", sc.calls, stagnationWindow+stagnationMaxWarnings-1, stagnationWindow, stagnationMaxWarnings)
	}
}
