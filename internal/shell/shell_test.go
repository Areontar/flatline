package shell

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunCaptures(t *testing.T) {
	r := Run(context.Background(), "echo hello", time.Second, 4096)
	if r.ExitCode != 0 || !strings.Contains(r.Stdout, "hello") {
		t.Fatalf("%+v", r)
	}
}

func TestRunExitCode(t *testing.T) {
	r := Run(context.Background(), "exit 3", time.Second, 4096)
	if r.ExitCode != 3 {
		t.Fatalf("exit code: %+v", r)
	}
}

func TestRunTimeout(t *testing.T) {
	r := Run(context.Background(), "sleep 5", 100*time.Millisecond, 4096)
	if !r.TimedOut {
		t.Fatalf("expected timeout: %+v", r)
	}
}

func TestRunTruncates(t *testing.T) {
	r := Run(context.Background(), "yes x | head -c 5000", time.Second, 100)
	if len(r.Stdout) > 300 || !strings.Contains(r.Stdout, "truncated") {
		t.Fatalf("truncate: len=%d", len(r.Stdout))
	}
}
