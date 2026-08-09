package model

type Ladder struct {
	models  []string
	factory func(string) Chat
	idx     int
	cur     Chat
}

func NewLadder(models []string, factory func(string) Chat, start int) *Ladder {
	if len(models) == 0 {
		models = []string{"default"}
	}
	if start < 0 || start >= len(models) {
		start = 0
	}
	l := &Ladder{models: models, factory: factory, idx: start}
	l.cur = factory(models[start])
	return l
}
func (l *Ladder) Current() Chat  { return l.cur }
func (l *Ladder) Model() string  { return l.models[l.idx] }
func (l *Ladder) Escalate() bool {
	if l.idx+1 >= len(l.models) {
		return false
	}
	l.idx++
	l.cur = l.factory(l.models[l.idx])
	return true
}
