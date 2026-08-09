package mock

import (
	"context"
	"testing"
	"time"

	"github.com/Areontar/flatline/internal/loop"
	"github.com/Areontar/flatline/internal/model"
	"github.com/Areontar/flatline/internal/shell"
	"github.com/Areontar/flatline/internal/sidecar"
)

type scripted struct {
	steps []model.Response
	i     int
}

func (s *scripted) Model() string { return "scripted" }
func (s *scripted) Complete(context.Context, []model.Message, []model.Tool) (model.Response, error) {
	r := s.steps[s.i]
	if s.i < len(s.steps)-1 {
		s.i++
	}
	return r, nil
}

func TestEndToEndSolve(t *testing.T) {
	flag := "flag{end_to_end}"
	target := NewTarget(flag)
	defer target.Close()
	sc := NewSidecarServer(flag)
	defer sc.Close()

	client := sidecar.New(sc.URL, "chal-1")
	chat := &scripted{steps: []model.Response{
		{Content: "Thought: fetch\nAction: run_shell\nAction Input: curl -s " + target.URL + "/secret"},
		{Content: "Thought: got it\nAction: submit_flag\nAction Input: " + flag},
	}}
	ladder := model.NewLadder([]string{"scripted"}, func(string) model.Chat { return chat }, 0)
	sh := loop.ShellFunc(func(c context.Context, cmd string) shell.Result {
		return shell.Run(c, cmd, 10*time.Second, 8000)
	})
	eng := loop.NewEngine(ladder, client, sh, loop.Spec{FlagRegex: `flag\{.+\}`}, 10, nil)

	solved, _ := eng.Run(context.Background(), "solve")
	if !solved {
		t.Fatal("expected end-to-end solve")
	}
}
