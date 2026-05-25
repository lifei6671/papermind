package migration

import (
	"errors"
	"testing"
)

func TestRunnerPublishesDoneStateAfterSuccessfulMigration(t *testing.T) {
	var runner *Runner
	runner = NewRunner(func() error {
		if status := runner.Status(); status.State != StateRunning {
			t.Fatalf("state during migration = %s", status.State)
		}
		return nil
	})

	if err := runner.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if status := runner.Status(); status.State != StateDone {
		t.Fatalf("state after migration = %s", status.State)
	}
}

func TestRunnerPublishesFailedStateAfterMigrationError(t *testing.T) {
	expectedErr := errors.New("broken migration")
	runner := NewRunner(func() error {
		return expectedErr
	})

	if err := runner.Run(); !errors.Is(err, expectedErr) {
		t.Fatalf("Run() error = %v", err)
	}
	status := runner.Status()
	if status.State != StateFailed {
		t.Fatalf("state after failed migration = %s", status.State)
	}
	if status.Error == "" {
		t.Fatalf("status error is empty")
	}
}
