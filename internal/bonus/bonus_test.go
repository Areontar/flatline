package bonus

import (
	"os"
	"testing"
)

func TestScanEnvAndFiles(t *testing.T) {
	got := Scan(
		[]string{"HOME=/root", "SECRET=flag{env_bonus}", "X=nope"},
		func(p string) (string, error) {
			if p == "/flag.txt" {
				return "here is HAL{file_bonus}\n", nil
			}
			return "", os.ErrNotExist
		},
	)
	if len(got) != 2 {
		t.Fatalf("want 2 flags, got %v", got)
	}
}

func TestScanRawFlagEnvVars(t *testing.T) {
	readNothing := func(p string) (string, error) {
		return "", os.ErrNotExist
	}

	got := Scan(
		[]string{"BONUS_FLAG=abc123", "FLAG_1=zzz", "OTHER=nope"},
		readNothing,
	)

	// Check that abc123 and zzz are in the results
	foundAbc123 := false
	foundZzz := false
	foundNope := false

	for _, flag := range got {
		if flag == "abc123" {
			foundAbc123 = true
		}
		if flag == "zzz" {
			foundZzz = true
		}
		if flag == "nope" {
			foundNope = true
		}
	}

	if !foundAbc123 {
		t.Errorf("wanted abc123 in results, got %v", got)
	}
	if !foundZzz {
		t.Errorf("wanted zzz in results, got %v", got)
	}
	if foundNope {
		t.Errorf("did not want nope in results, got %v", got)
	}
}
