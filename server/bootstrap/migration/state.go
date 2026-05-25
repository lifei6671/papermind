package migration

import (
	"errors"
	"sync"
)

type State string

const (
	StateIdle    State = "idle"
	StateRunning State = "running"
	StateDone    State = "done"
	StateFailed  State = "failed"
)

var ErrMigrationRunning = errors.New("migration is already running")

type Status struct {
	State State
	Error string
}

type Runner struct {
	mu     sync.RWMutex
	status Status
	run    func() error
}

func NewRunner(run func() error) *Runner {
	return &Runner{
		status: Status{State: StateIdle},
		run:    run,
	}
}

func (r *Runner) Status() Status {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.status
}

func (r *Runner) Run() error {
	r.mu.Lock()
	if r.status.State == StateRunning {
		r.mu.Unlock()
		return ErrMigrationRunning
	}
	r.status = Status{State: StateRunning}
	r.mu.Unlock()

	// 启动迁移期间对外暴露 running 状态，HTTP 层据此返回系统升级中页面，避免前端白屏或接口误报未知错误。
	err := r.run()

	r.mu.Lock()
	defer r.mu.Unlock()
	if err != nil {
		r.status = Status{State: StateFailed, Error: err.Error()}
		return err
	}
	r.status = Status{State: StateDone}
	return nil
}
