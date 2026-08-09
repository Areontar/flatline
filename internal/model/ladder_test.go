// ladder_test.go
package model

import (
	"context"
	"testing"
)

type fakeChat struct{ name string }

func (f fakeChat) Complete(context.Context, []Message, []Tool) (Response, error) { return Response{}, nil }
func (f fakeChat) Model() string                                                 { return f.name }

func TestLadderEscalation(t *testing.T) {
	l := NewLadder([]string{"s", "m", "l"}, func(n string) Chat { return fakeChat{n} }, 0)
	if l.Model() != "s" {
		t.Fatal("start")
	}
	if !l.Escalate() || l.Model() != "m" {
		t.Fatal("escalate to m")
	}
	if !l.Escalate() || l.Model() != "l" {
		t.Fatal("escalate to l")
	}
	if l.Escalate() {
		t.Fatal("should not escalate past top")
	}
}

func TestLadderStartRung(t *testing.T) {
	l := NewLadder([]string{"s", "m", "l"}, func(n string) Chat { return fakeChat{n} }, 1)
	if l.Model() != "m" {
		t.Fatalf("start rung: %s", l.Model())
	}
}
