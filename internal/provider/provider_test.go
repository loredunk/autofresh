package provider

import (
	"context"
	"errors"
	"strings"
	"testing"

	"autofresh/internal/logging"
)

type fakeExecutor struct {
	runErrors  map[string]error
	lookErrors map[string]error
	replies    map[string]string
	commands   []Command
}

func (f *fakeExecutor) Run(_ context.Context, command Command) (ExecutionResult, error) {
	f.commands = append(f.commands, command)
	return ExecutionResult{
		Provider: command.Provider,
		Reply:    f.replies[command.Provider],
	}, f.runErrors[command.Provider]
}

func (f *fakeExecutor) LookPath(name string) error {
	return f.lookErrors[name]
}

type fakeLogger struct {
	entries []logging.Entry
}

func (l *fakeLogger) Log(entry logging.Entry) error {
	l.entries = append(l.entries, entry)
	return nil
}

func TestBuildCommandForAll(t *testing.T) {
	t.Parallel()

	cmds, err := BuildCommands("all")
	if err != nil {
		t.Fatal(err)
	}

	if len(cmds) != 2 {
		t.Fatalf("got %d commands", len(cmds))
	}

	if !slicesEqual(cmds[0].Args, []string{"exec", "--skip-git-repo-check", "--ephemeral", "ok"}) {
		t.Fatalf("unexpected codex args: %#v", cmds[0].Args)
	}

	if !slicesEqual(cmds[1].Args, []string{"-p", "ok"}) {
		t.Fatalf("unexpected claude args: %#v", cmds[1].Args)
	}
}

func TestBuildCommandForClaudeUsesPrintMode(t *testing.T) {
	t.Parallel()

	cmds, err := BuildCommands("claude")
	if err != nil {
		t.Fatal(err)
	}

	if len(cmds) != 1 {
		t.Fatalf("got %d commands", len(cmds))
	}

	if !slicesEqual(cmds[0].Args, []string{"-p", "ok"}) {
		t.Fatalf("unexpected claude args: %#v", cmds[0].Args)
	}
}

func TestRunnerContinuesAfterSingleFailure(t *testing.T) {
	executor := &fakeExecutor{
		runErrors: map[string]error{
			"codex":  errors.New("boom"),
			"claude": nil,
		},
		replies: map[string]string{
			"claude": "OK",
		},
	}
	logger := &fakeLogger{}
	runner := NewRunner(executor, logger)

	_, err := runner.Run(context.Background(), "all", "manual")
	if err == nil {
		t.Fatal("expected error")
	}

	if len(executor.commands) != 2 {
		t.Fatalf("got %d commands", len(executor.commands))
	}

	if len(logger.entries) != 2 {
		t.Fatalf("got %d log entries", len(logger.entries))
	}

	if strings.Contains(err.Error(), "exit status") {
		t.Fatalf("error should be human readable, got %q", err.Error())
	}

	if !strings.Contains(err.Error(), "Codex command failed") {
		t.Fatalf("expected provider specific text, got %q", err.Error())
	}
}

func TestRunnerReturnsRepliesForSuccessfulCommands(t *testing.T) {
	executor := &fakeExecutor{
		replies: map[string]string{
			"codex": "OK.",
		},
	}
	runner := NewRunner(executor, &fakeLogger{})

	results, err := runner.Run(context.Background(), "codex", "manual")
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("got %d results", len(results))
	}

	if results[0].Reply != "OK." {
		t.Fatalf("got reply %q", results[0].Reply)
	}
}

func TestRunnerTimeoutIsHumanReadable(t *testing.T) {
	executor := &fakeExecutor{
		runErrors: map[string]error{
			"claude": context.DeadlineExceeded,
		},
	}
	runner := NewRunner(executor, &fakeLogger{})

	_, err := runner.Run(context.Background(), "claude", "manual")
	if err == nil {
		t.Fatal("expected timeout error")
	}

	if strings.Contains(err.Error(), "signal: killed") {
		t.Fatalf("timeout should be human readable, got %q", err.Error())
	}

	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout text, got %q", err.Error())
	}
}

func slicesEqual(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}

	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}

	return true
}
