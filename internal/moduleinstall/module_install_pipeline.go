package moduleinstall

import (
	"context"
	"errors"
)

type rollbackFunc func(context.Context) error

type rollbackStep struct {
	name string
	fn   rollbackFunc
}

type rollbackStack struct {
	logFn InstallLogFn
	steps []rollbackStep
}

func newRollbackStack(logFn InstallLogFn) *rollbackStack {
	return &rollbackStack{
		logFn: logFn,
		steps: make([]rollbackStep, 0, 4),
	}
}

func (s *rollbackStack) Add(name string, fn rollbackFunc) {
	if s == nil || fn == nil {
		return
	}
	s.steps = append(s.steps, rollbackStep{name: name, fn: fn})
}

func (s *rollbackStack) Run(ctx context.Context) error {
	if s == nil || len(s.steps) == 0 {
		return nil
	}

	var outErr error
	for i := len(s.steps) - 1; i >= 0; i-- {
		step := s.steps[i]
		logInstall(s.logFn, "rollback", "rollback step start=%s", step.name)
		err := step.fn(ctx)
		if err != nil {
			logInstall(s.logFn, "rollback", "[error] rollback step failed=%s err=%v", step.name, err)
			outErr = errors.Join(outErr, err)
			continue
		}
		logInstall(s.logFn, "rollback", "rollback step completed=%s", step.name)
	}
	return outErr
}

func (s *rollbackStack) Clear() {
	if s == nil {
		return
	}
	s.steps = nil
}
