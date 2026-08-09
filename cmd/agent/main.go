package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Areontar/flatline/internal/bonus"
	"github.com/Areontar/flatline/internal/config"
	"github.com/Areontar/flatline/internal/loop"
	"github.com/Areontar/flatline/internal/model"
	"github.com/Areontar/flatline/internal/router"
	"github.com/Areontar/flatline/internal/shell"
	"github.com/Areontar/flatline/internal/sidecar"
	"github.com/Areontar/flatline/internal/skills"
)

func main() {
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	var logMu sync.Mutex
	logln := func(s string) {
		logMu.Lock()
		defer logMu.Unlock()
		fmt.Fprintln(out, s)
		out.Flush()
	}

	// USER ID must be the first stdout line, flushed, within 30s.
	fmt.Fprintf(out, "USER ID: %s\n", os.Getenv("HAL_USER_ID"))
	out.Flush()

	// Log the NAMES of injected HAL_* env vars (names only - values may be flags).
	logHalEnvNames(logln)

	cfg, err := config.Load(os.Getenv)
	if err != nil {
		logln("config error: " + err.Error())
		os.Exit(1)
	}
	cfg.Targets = config.ParseTargets(os.Environ())

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	// Heartbeat - platform kills after 1m of stdout silence.
	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				logln("[heartbeat] working…")
			}
		}
	}()

	defer func() {
		if r := recover(); r != nil {
			logln(fmt.Sprintf("[panic recovered] %v", r))
		}
	}()

	// Sidecar is the primary submit + graceful-done path (no MCP needed).
	sc := sidecar.New(os.Getenv("SIDECAR_URL"), cfg.Challenge.ID)
	defer func() {
		shutCtx, c2 := context.WithTimeout(context.Background(), 10*time.Second)
		defer c2()
		if err := sc.Done(shutCtx); err != nil {
			logln("sidecar done error: " + err.Error())
		}
	}()

	// 1) Bonus/starter flags from env first - validates the submit pipeline.
	if bonus.Grab(ctx, sc, os.Environ()) {
		logln("bonus/starter flag submitted OK (pipeline validated)")
	}

	// 2) Deterministic route → specialist; ladder starts at the specialist's rung.
	prof := router.Route(cfg.Challenge.Category)

	skillsDir := os.Getenv("SKILLS_DIR")
	if skillsDir == "" {
		skillsDir = "/skills"
	}
	if cat := skills.Catalog(skillsDir); cat != "" {
		prof.System = prof.System + "\n\n" + cat
		logln("loaded skills catalog into system prompt")
	}

	ladder := model.NewLadder(cfg.ModelLadder,
		func(name string) model.Chat { return model.NewOpenAI(cfg.OpenAIBaseURL, name) },
		prof.StartRung)
	logln(fmt.Sprintf("challenge=%q category=%q model=%s", cfg.Challenge.Name, cfg.Challenge.Category, ladder.Model()))

	maxObs := 4000
	source := "default"
	if v := os.Getenv("MAX_OBS_BYTES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxObs = n
			source = "MAX_OBS_BYTES"
		}
	} else if ctxStr := os.Getenv("HAL_AGENT_MODEL_CTX_WINDOW"); ctxStr != "" {
		if ctxTokens, err := strconv.Atoi(ctxStr); err == nil && ctxTokens > 0 {
			derived := ctxTokens * 4 / 10
			if derived < 1500 {
				derived = 1500
			} else if derived > 12000 {
				derived = 12000
			}
			maxObs = derived
			source = fmt.Sprintf("HAL_AGENT_MODEL_CTX_WINDOW=%d", ctxTokens)
		}
	}
	logln(fmt.Sprintf("observation cap: %d bytes (%s)", maxObs, source))
	sh := loop.ShellFunc(func(c context.Context, cmd string) shell.Result {
		return shell.Run(c, cmd, 45*time.Second, maxObs)
	})
	eng := loop.NewEngine(ladder, sc, sh,
		loop.Spec{System: prof.System, FlagRegex: prof.FlagRegex}, cfg.MaxSteps, logln)

	solved, usedModel := eng.Run(ctx, buildPrompt(cfg))
	logln(fmt.Sprintf("run complete: solved=%v model=%s", solved, usedModel))
}

func logHalEnvNames(logln func(string)) {
	var names []string
	for _, kv := range os.Environ() {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			if k := kv[:i]; strings.HasPrefix(k, "HAL_") {
				names = append(names, k)
			}
		}
	}
	sort.Strings(names)
	logln("injected HAL_* env vars: " + strings.Join(names, ", "))
}

func buildPrompt(cfg config.Config) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Challenge: %s [%s]\n%s\n", cfg.Challenge.Name, cfg.Challenge.Category, cfg.Challenge.Description)
	for name, tgt := range cfg.Targets {
		if name == "" {
			fmt.Fprintf(&b, "Target: %s:%s\n", tgt.IP, tgt.Port)
		} else {
			fmt.Fprintf(&b, "Target %s: %s:%s\n", name, tgt.IP, tgt.Port)
		}
	}
	b.WriteString("Find and submit the flag.")
	return b.String()
}
