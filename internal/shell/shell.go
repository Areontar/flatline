package shell

import (
	"context"
	"os/exec"
	"time"
)

type Result struct {
	Stdout   string
	ExitCode int
	TimedOut bool
}

// Run executes command via `sh -c`, capturing combined stdout+stderr, enforcing
// a per-command timeout, and truncating output to maxBytes (keeping the tail,
// where flags/errors usually appear).
func Run(ctx context.Context, command string, timeout time.Duration, maxBytes int) Result {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, "sh", "-c", command)
	out, err := cmd.CombinedOutput()
	r := Result{Stdout: truncateTail(string(out), maxBytes), ExitCode: 0}
	if cctx.Err() == context.DeadlineExceeded {
		r.TimedOut = true
		r.ExitCode = -1
	} else if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			r.ExitCode = ee.ExitCode()
		} else {
			r.ExitCode = -1
			r.Stdout += "\n[exec error: " + err.Error() + "]"
		}
	}
	return r
}

func truncateTail(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	return "…[truncated " + itoa(len(s)-maxBytes) + " bytes]…\n" + s[len(s)-maxBytes:]
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
