package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/Areontar/flatline/internal/mcp"
	"github.com/Areontar/flatline/internal/model"
	"github.com/Areontar/flatline/internal/shell"
)

// maxConsecutiveModelErrors is the number of consecutive model-call failures
// (HTTP 500s, timeouts, or any other backend error) the loop tolerates before
// giving up. Each Complete call already retries internally (~4x), so this is
// ~12 total backend attempts -- plenty to distinguish a transient blip from a
// genuinely wedged backend, while still giving up promptly on the latter so
// main's deferred sc.Done() frees the shared queue slot instead of holding it
// for the whole step budget.
const maxConsecutiveModelErrors = 3

// maxAutoSubmitsPerRun caps how many DISTINCT flag-shaped strings found in
// command output the loop will auto-submit, so a page full of decoy/example
// flags can't burn a challenge's whole attempt allowance.
const maxAutoSubmitsPerRun = 12

// Stagnation guard: if the recent action window collapses to too few distinct
// actions (a weak model cycling dead-end commands or emitting persistent
// garbage), give up after a couple of warnings rather than burning the whole
// step budget and holding the shared queue slot. Complements repeat-detection
// (exact repeats) and the model-error circuit breaker (backend failures).
const (
	stagnationWindow      = 15
	stagnationMinDistinct = 4
	stagnationMaxWarnings = 2
)

// possibleFlagRe matches flag-SHAPED strings for OBSERVABILITY only: a short
// alnum/underscore prefix + braces whose contents look flag-like (contiguous,
// no ':', ';', whitespace, or nested braces - which rules out CSS/JS noise).
// When such a string does NOT match the active flag regex, we log it so real
// run logs surface flag formats our submit pattern would miss (e.g. a partner
// CTF's prefix), without submitting CSS/JS false positives.
var possibleFlagRe = regexp.MustCompile(`[A-Za-z][A-Za-z0-9_]{1,23}\{[^{}:;\s]{4,200}\}`)

type Submitter interface {
	SubmitFlag(ctx context.Context, flag string) (mcp.SubmitResult, error)
}
type ShellFunc func(ctx context.Context, cmd string) shell.Result

type Spec struct {
	System    string
	FlagRegex string
}

var shellTool = model.Tool{
	Name: "run_shell", Description: "Run one shell command in the sandbox.",
	Parameters: map[string]any{"type": "object",
		"properties": map[string]any{"command": map[string]any{"type": "string"}},
		"required":   []string{"command"}},
}
var submitTool = model.Tool{
	Name: "submit_flag", Description: "Submit a candidate flag you have actually seen.",
	Parameters: map[string]any{"type": "object",
		"properties": map[string]any{"flag": map[string]any{"type": "string"}},
		"required":   []string{"flag"}},
}

type Engine struct {
	ladder   *model.Ladder
	sub      Submitter
	shell    ShellFunc
	spec     Spec
	maxSteps int
	log      func(string)
	flagRe   *regexp.Regexp
}

func NewEngine(l *model.Ladder, sub Submitter, sh ShellFunc, spec Spec, maxSteps int, log func(string)) *Engine {
	if log == nil {
		log = func(string) {}
	}
	re, err := regexp.Compile(spec.FlagRegex)
	if err != nil || spec.FlagRegex == "" {
		re = regexp.MustCompile(`(?i)(flag|ctf|hal)\{[^}]{1,120}\}`)
	}
	return &Engine{ladder: l, sub: sub, shell: sh, spec: spec, maxSteps: maxSteps, log: log, flagRe: re}
}

func (e *Engine) Run(ctx context.Context, userPrompt string) (bool, string) {
	msgs := []model.Message{{Role: "system", Content: e.spec.System}, {Role: "user", Content: userPrompt}}
	seenActions := map[string]bool{}
	wrongFlags := map[string]bool{}
	wrongGuesses := 0
	stalls := 0
	consecutiveModelErrors := 0
	autoSubmits := 0
	window := make([]string, 0, stagnationWindow)
	stagnationWarnings := 0
	flagWarned := map[string]bool{}
	totalPrompt, totalCompletion := 0, 0
	for step := 0; step < e.maxSteps; step++ {
		e.log(fmt.Sprintf("--- step %d/%d ---", step+1, e.maxSteps))
		msgs = compact(msgs, 12)
		if ctx.Err() != nil {
			return false, e.ladder.Model()
		}
		resp, err := e.ladder.Current().Complete(ctx, msgs, []model.Tool{shellTool, submitTool})
		if err != nil {
			consecutiveModelErrors++
			e.log("model error: " + err.Error())
			if consecutiveModelErrors >= maxConsecutiveModelErrors {
				e.log(fmt.Sprintf("giving up: model backend failed %d times in a row", consecutiveModelErrors))
				return false, e.ladder.Model()
			}
			time.Sleep(500 * time.Millisecond)
			continue
		}
		consecutiveModelErrors = 0
		totalPrompt += resp.PromptTokens
		totalCompletion += resp.CompletionTokens
		e.log(fmt.Sprintf("tokens: +%d in / +%d out (cumulative %d in / %d out)",
			resp.PromptTokens, resp.CompletionTokens, totalPrompt, totalCompletion))
		msgs = appendAssistant(msgs, resp)
		act, ok := ParseAction(resp)

		wkey := "<unparseable>"
		if ok {
			wkey = actionKey(act)
		}
		window = append(window, wkey)
		if len(window) > stagnationWindow {
			window = window[1:]
		}
		if len(window) == stagnationWindow {
			if distinctCount(window) < stagnationMinDistinct {
				stagnationWarnings++
				if stagnationWarnings >= stagnationMaxWarnings {
					e.log("giving up: action diversity collapsed (stagnation guard)")
					return false, e.ladder.Model()
				}
			} else {
				stagnationWarnings = 0
			}
		}

		if !ok {
			stalls++
			e.log("unparseable model output: " + preview(resp.Content, 300))
			msgs = e.appendNudge(msgs, resp,
				"Respond in exactly:\nThought: <reasoning>\nAction: run_shell OR submit_flag\nAction Input: <command or flag>")
			e.maybeEscalate(&stalls)
			continue
		}
		key := actionKey(act)
		if seenActions[key] && act.Kind == ActionShell {
			stalls++
			msgs = e.appendNudge(msgs, resp, "You already ran that exact command; its result is above. Try a different approach.")
			e.maybeEscalate(&stalls)
			continue
		}
		seenActions[key] = true

		switch act.Kind {
		case ActionShell:
			stalls = 0
			wrongGuesses = 0
			e.log("run_shell: " + oneLine(act.Command))
			e.log("$ " + act.Command)
			r := e.shell(ctx, act.Command)
			ts := ""
			if r.TimedOut {
				ts = " (timed out)"
			}
			e.log(fmt.Sprintf("[exit %d%s]", r.ExitCode, ts))
			if r.Stdout != "" {
				e.log(r.Stdout)
			}
			msgs = appendObservation(msgs, act, r.Stdout)
			e.logUnrecognizedFlags(r.Stdout, flagWarned)
			if act.ToolCallID != "" {
				msgs = answerExtraToolCalls(msgs, resp, act.ToolCallID)
			}
			if e.autoSubmitFromOutput(ctx, r.Stdout, wrongFlags, &autoSubmits) {
				return true, e.ladder.Model()
			}
		case ActionSubmit:
			if e.flagRe != nil && !e.flagRe.MatchString(act.Flag) {
				// Guarded: don't submit obvious non-flags; nudge instead.
				msgs = appendToolResult(msgs, act, "That does not look like a valid flag format; keep investigating.")
				if act.ToolCallID != "" {
					msgs = answerExtraToolCalls(msgs, resp, act.ToolCallID)
				}
				continue
			}
			if wrongFlags[act.Flag] {
				// Already tried and confirmed wrong; don't burn another
				// platform attempt on an exact repeat, but still treat it as
				// a wrong guess for stall/escalation accounting.
				wrongGuesses++
				stalls++
				msgs = appendToolResult(msgs, act, "That exact flag was already submitted and was incorrect. Do not resubmit it -- find a different flag.")
				if act.ToolCallID != "" {
					msgs = answerExtraToolCalls(msgs, resp, act.ToolCallID)
				}
				e.maybeEscalate(&stalls)
				continue
			}
			e.log("submit_flag: " + act.Flag)
			res, err := e.sub.SubmitFlag(ctx, act.Flag)
			if err != nil {
				msgs = appendToolResult(msgs, act, "submit error: "+err.Error())
				if act.ToolCallID != "" {
					msgs = answerExtraToolCalls(msgs, resp, act.ToolCallID)
				}
				continue
			}
			e.log(fmt.Sprintf("submit %q -> correct=%v already_solved=%v attempts_left=%d msg=%q",
				act.Flag, res.Correct, res.AlreadySolved, res.AttemptsLeft, res.Message))
			if res.Correct || res.AlreadySolved {
				e.log("SOLVED with model " + e.ladder.Model())
				return true, e.ladder.Model()
			}
			wrongFlags[act.Flag] = true
			wrongGuesses++
			stalls++
			note := "Incorrect flag."
			if wrongGuesses >= 2 {
				note += " Stop guessing - go find the actual flag in command output before submitting again."
			}
			msgs = appendToolResult(msgs, act, note)
			if act.ToolCallID != "" {
				msgs = answerExtraToolCalls(msgs, resp, act.ToolCallID)
			}
			e.maybeEscalate(&stalls)
		}
	}
	e.log("step budget exhausted")
	return false, e.ladder.Model()
}

// autoSubmitFromOutput scans command output for flag-format matches and submits
// any it has not already tried. Returns true if one is accepted (correct or
// already-solved). Shares the wrongFlags dedup set so a rejected auto-submit is
// never retried (by auto-submit OR by the model's own submit path).
func (e *Engine) autoSubmitFromOutput(ctx context.Context, output string, wrongFlags map[string]bool, count *int) bool {
	if e.flagRe == nil {
		return false
	}
	seen := map[string]bool{}
	for _, cand := range e.flagRe.FindAllString(output, -1) {
		if seen[cand] || wrongFlags[cand] {
			continue
		}
		seen[cand] = true
		if *count >= maxAutoSubmitsPerRun {
			return false
		}
		*count++
		res, err := e.sub.SubmitFlag(ctx, cand)
		if err != nil {
			continue
		}
		e.log(fmt.Sprintf("submit %q -> correct=%v already_solved=%v attempts_left=%d msg=%q",
			cand, res.Correct, res.AlreadySolved, res.AttemptsLeft, res.Message))
		if res.Correct || res.AlreadySolved {
			e.log("auto-captured flag from command output: " + cand)
			return true
		}
		wrongFlags[cand] = true // do not retry this exact string
	}
	return false
}

// logUnrecognizedFlags scans output for flag-SHAPED strings (possibleFlagRe)
// that do NOT match the active flag regex, and logs each distinct one once
// (via warned) as a possible unrecognized flag format. This is observability
// only -- it never submits anything.
func (e *Engine) logUnrecognizedFlags(output string, warned map[string]bool) {
	for _, cand := range possibleFlagRe.FindAllString(output, -1) {
		if warned[cand] || (e.flagRe != nil && e.flagRe.MatchString(cand)) {
			continue // already warned, or our pattern already recognizes it
		}
		warned[cand] = true
		e.log("⚠ possible unrecognized flag format in output (not submitted): " + cand)
	}
}

// preview collapses whitespace/newlines in s to single spaces and truncates
// to n runes with an ellipsis, for compact single-line log previews of
// otherwise-multiline model output.
func preview(s string, n int) string {
	s = strings.Join(strings.FieldsFunc(s, unicode.IsSpace), " ")
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// appendNudge answers any pending tool_calls on resp with a role:"tool"
// message carrying the nudge text (satisfying the OpenAI wire contract that
// every tool_call must be immediately answered by a matching tool result),
// falling back to a plain role:"user" message when resp carried no native
// tool calls (i.e. the plain-text ReAct fallback path).
func (e *Engine) appendNudge(msgs []model.Message, resp model.Response, text string) []model.Message {
	if len(resp.ToolCalls) > 0 {
		for _, tc := range resp.ToolCalls {
			msgs = append(msgs, model.Message{Role: "tool", ToolCallID: tc.ID, Content: text})
		}
		return msgs
	}
	return append(msgs, model.Message{Role: "user", Content: text})
}

func (e *Engine) maybeEscalate(stalls *int) {
	if *stalls >= 3 {
		if e.ladder.Escalate() {
			e.log("escalating to model " + e.ladder.Model())
		}
		*stalls = 0
	}
}

func appendAssistant(msgs []model.Message, r model.Response) []model.Message {
	return append(msgs, model.Message{Role: "assistant", Content: r.Content, ToolCalls: sanitizeToolCalls(r.ToolCalls)})
}

// sanitizeToolCalls replaces any tool_call whose arguments are not valid JSON
// with "{}", so a truncated/malformed tool call the model emitted cannot poison
// the conversation history - the model backend returns HTTP 500 trying to
// re-parse invalid tool-call arguments on every subsequent request.
func sanitizeToolCalls(tcs []model.ToolCall) []model.ToolCall {
	if len(tcs) == 0 {
		return tcs
	}
	out := make([]model.ToolCall, len(tcs))
	for i, tc := range tcs {
		if !json.Valid([]byte(tc.Arguments)) {
			tc.Arguments = "{}"
		}
		out[i] = tc
	}
	return out
}

// answerExtraToolCalls answers every tool_call in resp.ToolCalls other than
// handledID with a stub role:"tool" result. The OpenAI wire contract requires
// every tool_call emitted by an assistant message to be answered before the
// next request; when the model emits 2+ tool_calls in one turn, only the
// first actionable one is executed, so the rest must still get a matching
// tool result or the next request 400s.
func answerExtraToolCalls(msgs []model.Message, resp model.Response, handledID string) []model.Message {
	for _, tc := range resp.ToolCalls {
		if tc.ID != "" && tc.ID != handledID {
			msgs = append(msgs, model.Message{Role: "tool", ToolCallID: tc.ID,
				Content: "Only one action is processed per turn; this call was not executed. Issue it again next turn if still needed."})
		}
	}
	return msgs
}
func appendObservation(msgs []model.Message, a Action, out string) []model.Message {
	if a.ToolCallID != "" {
		return append(msgs, model.Message{Role: "tool", ToolCallID: a.ToolCallID, Content: out})
	}
	return append(msgs, model.Message{Role: "user", Content: "Observation:\n" + out})
}
func appendToolResult(msgs []model.Message, a Action, out string) []model.Message {
	if a.ToolCallID != "" {
		return append(msgs, model.Message{Role: "tool", ToolCallID: a.ToolCallID, Content: out})
	}
	return append(msgs, model.Message{Role: "user", Content: "Observation: " + out})
}
func actionKey(a Action) string { return fmt.Sprintf("%d|%s|%s", a.Kind, a.Command, a.Flag) }

// distinctCount returns the number of distinct strings in xs.
func distinctCount(xs []string) int {
	seen := map[string]bool{}
	for _, x := range xs {
		seen[x] = true
	}
	return len(seen)
}
func oneLine(s string) string {
	if len(s) > 120 {
		s = s[:120] + "…"
	}
	return s
}

// compact keeps the system message, the first user message (the challenge
// prompt), and the most recent keepRecent messages, dropping the middle to
// bound context growth over long ReAct loops. If msgs is already short
// enough, it is returned unchanged. The tail's start is pulled backward past
// any role:"tool" message so the returned slice never begins with an
// orphaned tool result whose matching assistant tool_call was dropped.
func compact(msgs []model.Message, keepRecent int) []model.Message {
	if len(msgs) <= keepRecent+2 {
		return msgs
	}
	cut := len(msgs) - keepRecent
	for cut > 2 && msgs[cut].Role == "tool" { // never begin the tail on an unpaired tool result
		cut--
	}
	out := make([]model.Message, 0, 2+len(msgs)-cut)
	out = append(out, msgs[0], msgs[1])
	out = append(out, msgs[cut:]...)
	return out
}
