package provider

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"autofresh/internal/logging"
)

type Command struct {
	Provider string
	Name     string
	Args     []string
}

type ExecutionResult struct {
	Provider string
	Reply    string
}

func (c Command) Display() string {
	parts := []string{c.Name}
	parts = append(parts, c.Args...)
	return strings.Join(parts, " ")
}

type Executor interface {
	Run(ctx context.Context, command Command) (ExecutionResult, error)
	LookPath(name string) error
}

type EntryLogger interface {
	Log(entry logging.Entry) error
}

type Runner struct {
	Executor Executor
	Logger   EntryLogger
	Timeout  time.Duration
}

type OSExecutor struct{}

type CommandError struct {
	Command Command
	Cause   error
	Stderr  string
}

func (e CommandError) Error() string {
	if summary := summarizeStderr(e.Stderr); summary != "" {
		return summary
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return "command failed"
}

func BuildCommands(target string) ([]Command, error) {
	switch target {
	case "codex":
		return []Command{{
			Provider: "codex",
			Name:     "codex",
			Args:     []string{"exec", "--skip-git-repo-check", "--ephemeral", "ok"},
		}}, nil
	case "claude":
		return []Command{{
			Provider: "claude",
			Name:     "claude",
			Args:     []string{"-p", "ok"},
		}}, nil
	case "all":
		return []Command{
			{
				Provider: "codex",
				Name:     "codex",
				Args:     []string{"exec", "--skip-git-repo-check", "--ephemeral", "ok"},
			},
			{
				Provider: "claude",
				Name:     "claude",
				Args:     []string{"-p", "ok"},
			},
		}, nil
	default:
		return nil, fmt.Errorf("invalid target %q", target)
	}
}

func (OSExecutor) Run(ctx context.Context, command Command) (ExecutionResult, error) {
	cmd, outputPath, err := buildExecCommand(ctx, command)
	if err != nil {
		return ExecutionResult{}, err
	}
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		return ExecutionResult{}, err
	}
	defer devNull.Close()

	cmd.Stdin = devNull
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return ExecutionResult{}, context.DeadlineExceeded
		}
		return ExecutionResult{}, CommandError{
			Command: command,
			Cause:   err,
			Stderr:  stderr.String(),
		}
	}

	reply := strings.TrimSpace(stdout.String())
	if outputPath != "" {
		replyBytes, err := os.ReadFile(outputPath)
		if err == nil {
			reply = strings.TrimSpace(string(replyBytes))
		}
		_ = os.Remove(outputPath)
	}

	return ExecutionResult{
		Provider: command.Provider,
		Reply:    reply,
	}, nil
}

func (OSExecutor) LookPath(name string) error {
	_, err := exec.LookPath(name)
	return err
}

func NewRunner(executor Executor, logger EntryLogger) Runner {
	if executor == nil {
		executor = OSExecutor{}
	}

	return Runner{
		Executor: executor,
		Logger:   logger,
		Timeout:  45 * time.Second,
	}
}

func (r Runner) Run(ctx context.Context, target string, mode string) ([]ExecutionResult, error) {
	commands, err := BuildCommands(target)
	if err != nil {
		return nil, err
	}

	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 45 * time.Second
	}

	var failures []string
	results := make([]ExecutionResult, 0, len(commands))
	for _, command := range commands {
		runCtx, cancel := context.WithTimeout(ctx, timeout)
		result, err := r.Executor.Run(runCtx, command)
		cancel()

		if err != nil {
			message := humanizeFailure(command, err, timeout)
			failures = append(failures, message)
			r.log(command.Provider, mode, "failure", message)
			continue
		}

		results = append(results, result)
		logMessage := strings.TrimSpace(result.Reply)
		if logMessage == "" {
			logMessage = "ok"
		}
		r.log(command.Provider, mode, "success", logMessage)
	}

	if len(failures) > 0 {
		return results, errors.New(strings.Join(failures, "; "))
	}

	return results, nil
}

func (r Runner) Available(name string) error {
	return r.Executor.LookPath(name)
}

func (r Runner) log(provider string, mode string, result string, message string) {
	if r.Logger == nil {
		return
	}

	_ = r.Logger.Log(logging.Entry{
		Timestamp: time.Now(),
		Provider:  provider,
		Mode:      mode,
		Result:    result,
		Message:   message,
	})
}

func humanizeFailure(command Command, err error, timeout time.Duration) string {
	label := strings.Title(command.Provider)

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Sprintf("%s command timed out after %s. The CLI may be waiting on login, permissions, or network. Try `%s` manually.", label, timeout, command.Display())
	case errors.Is(err, exec.ErrNotFound):
		return fmt.Sprintf("%s CLI is not available in PATH. Expected command: `%s`.", label, command.Name)
	}

	summary := err.Error()
	var commandErr CommandError
	if errors.As(err, &commandErr) {
		if extracted := summarizeStderr(commandErr.Stderr); extracted != "" {
			summary = extracted
		}
	}

	summary = strings.TrimSpace(summary)
	if summary == "" || strings.Contains(summary, "exit status") {
		summary = "The CLI returned an error before completing the keepalive request."
	}

	return fmt.Sprintf("%s command failed. %s Try `%s` manually.", label, summary, command.Display())
}

func summarizeStderr(stderr string) string {
	lines := strings.Split(stderr, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		lower := strings.ToLower(line)
		if strings.HasPrefix(line, "Usage:") || strings.HasPrefix(lower, "tip:") {
			continue
		}
		if strings.Contains(lower, "exit status") {
			continue
		}
		if strings.Contains(line, " WARN ") {
			continue
		}

		return line
	}

	return ""
}

func buildExecCommand(ctx context.Context, command Command) (*exec.Cmd, string, error) {
	args := append([]string{}, command.Args...)
	outputPath := ""

	if command.Provider == "codex" {
		tempFile, err := os.CreateTemp("", "autofresh-codex-*.txt")
		if err != nil {
			return nil, "", err
		}
		outputPath = tempFile.Name()
		if err := tempFile.Close(); err != nil {
			return nil, "", err
		}

		args = append(args[:len(args)-1], append([]string{"-o", outputPath}, args[len(args)-1:]...)...)
	}

	return exec.CommandContext(ctx, command.Name, args...), outputPath, nil
}
