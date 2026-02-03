// Package testbed provides internal test widgets for the testing framework.
package testbed

import (
	"fmt"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/widgets"
)

// Counter is a stateful widget that displays a count and increments on tap.
type Counter struct {
	Initial int
	OnTap   func(count int)
}

func (c Counter) CreateElement() core.Element {
	return core.NewStatefulElement(c, nil)
}

func (c Counter) Key() any { return nil }

func (c Counter) CreateState() core.State {
	return &counterState{}
}

type counterState struct {
	core.StateBase
	count int
	onTap func(int)
}

func (s *counterState) InitState() {
	w := s.Element().Widget().(Counter)
	s.count = w.Initial
	s.onTap = w.OnTap
}

func (s *counterState) Build(ctx core.BuildContext) core.Widget {
	return widgets.GestureDetector{
		OnTap: func() {
			s.SetState(func() {
				s.count++
			})
			if s.onTap != nil {
				s.onTap(s.count)
			}
		},
		Child: widgets.Text{Content: fmt.Sprintf("%d", s.count)},
	}
}

func (s *counterState) DidUpdateWidget(oldWidget core.StatefulWidget) {
	if w, ok := s.Element().Widget().(Counter); ok {
		s.onTap = w.OnTap
	}
}
