```text
   █████ █      ███  █████ █     █████ █   █ █████
   █     █     █   █   █   █       █   ██  █ █
   ████  █     █████   █   █       █   █ █ █ ████
   █     █     █   █   █   █       █   █  ██ █
   █     █████ █   █   █   █████ █████ █   █ █████
   ────────────────────╱╲──────────────────────────  ░▒▓█
```

# FLATLINE

**A fault-tolerant CTF-solving agent that drives small, local language models to capture flags entirely on their own**, under a metered budget. Debuted at **DEF CON 34 · AI Village · HALCTF**.

HALCTF ("Hostile Autonomous Layer CTF") is an agentic security competition: you don't solve challenges by hand, you ship a container that solves them autonomously. Points are *higher* for smaller models and *lower* for chatty ones, so the score comes down to the quality of the harness around the model. FLATLINE is that harness.

---

## Design

A 4B-parameter model will truncate commands, spam dead ends, and emit malformed tool calls. In a shared, queued competition, each of those **wastes a whole detonation**. FLATLINE puts the reliability in the harness so a single bad model turn doesn't cost the whole run:

> **The harness owns reliability.**

Everything that *can* be made deterministic (routing, flag capture, budget control, failure recovery, wire-protocol correctness) runs in tested Go code, so the small model only has to reason about the next shell command. When it gets stuck, it stops early and frees its shared queue slot instead of spending the rest of the budget.

---

## Highlights

- 🧠 **Hybrid ReAct loop**: prefers native tool-calls and falls back to a plain-text `Thought/Action/Observation` protocol, so it works across small open models, including ones that can't reliably emit structured tool calls.
- 🎯 **Deterministic category routing**: the challenge category is an env var, so specialist selection costs **zero model tokens** (no LLM router).
- 🏁 **Autonomous flag capture**: the harness auto-submits any flag it *sees* in command output, so a solve never hinges on the weak model correctly deciding to submit. Env-injected flags are grabbed without spending any tokens.
- 🛡️ **A real safety net**: anti-poison tool-call sanitization, a wedged-backend circuit breaker, a low-diversity stagnation guard, exact-repeat detection, wrong-flag de-duplication, and flag-format hygiene. Every one of them frees the shared queue slot early rather than wasting the run.
- 🔬 **Deep observability**: per-step logs stream command output, submit outcomes, token spend, give-up reasons, and a `⚠ possible unrecognized flag` signal that surfaces novel flag formats *without* submitting CSS/JS noise. Logs cost zero model tokens.
- ⚙️ **Adaptive to the platform**: auto-detects the selected model and its context window from injected env vars, then sizes its own observation window, step budget, and multi-target map accordingly. All fallbacks are graceful, so local testing is unaffected.
- 🥷 **Two attack modes**: classic red-team (web / recon / pwn-rev / forensics-stego / AD-Windows / password) *and* prompt-attack (socially engineering or prompt-injecting a target chatbot/agent).
- 📦 **Tiny, dependency-lean**: a static Go binary of a few MB. The standard library does the harness; the offensive tooling lives in the container image.
- ✅ **Tested end-to-end, offline**: an in-process mock of the platform's sidecar plus a planted-flag target drive the *real* loop through a full solve, so you can iterate without burning a detonation.

---

## Architecture

```
                       injected env (challenge, target, model, endpoints)
                                        │
   USER ID ✔  ── heartbeat ──►  ┌───────▼────────┐
   (30s lint gate)              │   cmd/agent    │  bootstrap · panic-recovery · SIGTERM
                                │    (main)      │  logs HAL_* names (never values)
                                └───────┬────────┘
                                        │
        ┌───────────────┬───────────────┼────────────────┬───────────────┐
        ▼               ▼               ▼                ▼               ▼
   bonus grab      router →        model.Ladder      sidecar          skills
  (env + files)   specialist     (OpenAI client,   (POST /submit,   (/skills catalog
   auto-submit    (deterministic  retry/backoff,    POST /done,      injected into
                   per category)   escalation seam)  primary path)    the prompt)
        │               │               │                │
        └───────────────┴──────►  ┌──────▼───────┐ ◄──────┘
                                  │  loop.Engine  │  hybrid ReAct: tool-call OR plain-text
                                  │               │  repeat-detect · wrong-flag dedup
                                  │               │  circuit-breaker · stagnation guard
                                  │               │  auto-submit observed flags
                                  │               │  history compaction (tool-pair safe)
                                  └──────┬────────┘
                                    run_shell │ submit_flag
                                         ▼
                              sandboxed shell (timeout + tail-truncation)
                                         ▼
                            challenge target (HAL_TARGET_IP:PORT)
```

**Flow:** boot → grab env/starter flags (validates the submit pipeline) → route to a specialist by category → drive the model in the hybrid loop → run shell tools against the target → capture the flag (model-submitted *or* auto-captured from output) → submit via the sidecar → free the queue slot with `/done`. A heartbeat streams throughout so a long shell command or model call never looks hung.

---

## The reliability engineering

Each is a small, independently-tested Go unit targeting a specific failure mode:

| Safeguard | Failure it prevents |
|---|---|
| **Malformed tool-call sanitization** | A truncated tool call poisoning the conversation: the model backend 500s on *every* later request and the run dies. (Observed killing the reference agent.) |
| **Tool-call/result pairing** (nudges + compaction) | An unanswered or orphaned `tool_call` triggering a 400 that burns the budget. |
| **Consecutive-error circuit breaker** | Retrying a wedged/overloaded backend for 40 steps while holding a shared queue slot. |
| **Stagnation guard** | A model cycling through a handful of different-but-useless commands (which exact-repeat detection misses). |
| **Exact-repeat detection + wrong-flag dedup** | Re-running the identical command or re-submitting a known-wrong flag (wastes capped attempts). |
| **Flag-format hygiene + observability** | Submitting hallucinated junk, while *logging* flag-shaped strings we didn't recognize so real evidence drives detection tuning. |
| **Auto-submit from output** | A capture failing because the weak model saw the flag but didn't submit it. |

---

## Tech stack

- **Go 1.22+** (built and tested on the 1.26 toolchain): a static, `CGO_ENABLED=0` binary.
- **Standard library only** for the harness: no third-party imports, so there is full control over the wire protocol (essential for the hybrid tool-call handling). One dependency is *permitted and staged*, the official **[Model Context Protocol Go SDK](https://github.com/modelcontextprotocol/go-sdk)**, for the Streamable-HTTP MCP transport; it stays behind a `Transport` interface and is deferred until the endpoint format is confirmed at the event.
- **OpenAI-compatible chat completions**: a hand-rolled HTTP client (retry + jittered backoff, token accounting) pointed at the platform's model proxy.
- **MCP (Model Context Protocol)**: the platform's tool endpoint is a Streamable-HTTP MCP server; the critical submit/done path uses the sidecar's plain HTTP, with MCP reserved for richer tools (hints/status).
- **OCI / Docker container**: a small offensive-tooling image (curated separately) hosts the static binary; skills are self-describing playbooks under `/skills` that the harness discovers and injects into the prompt at boot.
- **Engineering process**: spec → plan → **subagent-driven, test-first development**, every task independently reviewed, plus a whole-branch review that caught and fixed (with regression tests) several cross-cutting defects.

---

## Project layout

```
cmd/agent/          bootstrap, heartbeat, orchestration, graceful exit
internal/
  config/           env → typed config (endpoints, challenge, targets, model, budgets)
  model/            Chat interface · OpenAI-compatible client · escalation ladder
  loop/             the hybrid ReAct engine + all safeguards + observability
  router/           deterministic category → specialist
  specialists/      per-category system prompts (web, recon, pwn-rev, forensics-stego,
                    ad-windows, password, protocol, prompt-attack, generic)
  challenges/       optional per-challenge strategy channel (see note below)
  bonus/            env/file flag scanner (pipeline smoke-test)
  sidecar/          plain-HTTP /submit + /done, the primary critical path
  mcp/              MCP client behind a Transport seam (richer tools, deferred wiring)
  skills/           reads /skills/*/SKILL.md and injects a catalog into the prompt
  mock/             in-process sidecar + planted-flag target for offline end-to-end tests
docs/               reference material (Go MCP SDK cheat-sheet)
```

> **Per-challenge hints:** `internal/challenges/challenges_data.json` is a **stub you fill in**.
> It maps a challenge name/slug/ID to an extra guidance string appended to the specialist
> prompt at runtime, a place to inject expert knowledge for a specific challenge when no
> human is in the loop. It ships empty (one self-documenting example entry); set it to `{}`
> to disable the feature entirely. The agent's real capability lives in the specialists and
> skills, so this channel is strictly optional.

---

## Build & test

```bash
# Test the whole harness (unit + end-to-end integration, race detector)
go test ./... -race

# Build the static binary
CGO_ENABLED=0 go build -ldflags="-s -w" -o agent ./cmd/agent
```

The agent is configured entirely through environment variables (all with sane fallbacks):

| Var | Purpose |
|---|---|
| `OPENAI_BASE_URL` | OpenAI-compatible model endpoint (**required**) |
| `MODEL` / `MODEL_LADDER` / `HAL_AGENT_MODEL` | model selection (first set wins; platform-injected `HAL_AGENT_MODEL` picked up automatically) |
| `HAL_AGENT_MODEL_CTX_WINDOW` | context window → auto-sizes the observation cap |
| `MAX_STEPS` | step budget (default 80) |
| `MAX_OBS_BYTES` | shell-output cap override |
| `SIDECAR_URL` | flag submit/done endpoint (default `http://127.0.0.1:9000`) |
| `SKILLS_DIR` | skill playbook directory (default `/skills`) |
| `HAL_CHALLENGE_*`, `HAL_TARGET_*` | challenge + target details (platform-injected) |

**Local testing** against a real small model: point `OPENAI_BASE_URL` at [LM Studio](https://lmstudio.ai) (`http://localhost:1234/v1`) with a Qwen/Llama model loaded, set `MODEL` to it, and run. The offline mock harness covers the rest.

---

## Status

The harness is complete, whole-branch reviewed, and green under the race detector. The offensive-tooling container image and skill playbooks are developed in parallel and merged in. The observability layer feeds tuning from real run logs.

*FLATLINE · Built for DEF CON 34 · AI Village · HALCTF.*

---

## License

Released under the [MIT License](LICENSE).
