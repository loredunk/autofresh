package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"autofresh/internal/config"
	"autofresh/internal/logging"
	"autofresh/internal/platform"
	"autofresh/internal/provider"
	"autofresh/internal/schedule"
)

type ConfigStore interface {
	Load() (config.Config, error)
	Save(cfg config.Config) error
	Delete() error
	Exists() (bool, error)
	Path() string
}

type Platform interface {
	Install(binaryPath string, pathValue string, times []schedule.TimeOfDay) error
	Remove() error
	Status() (string, error)
}

type Runner interface {
	Run(ctx context.Context, target string, mode string) ([]provider.ExecutionResult, error)
	Available(name string) error
}

type Service struct {
	Store          ConfigStore
	Platform       Platform
	Runner         Runner
	ExecutablePath func() (string, error)
	PathValue      func() string
	LogPath        func() (string, error)
}

func NewDefaultService() (*Service, error) {
	configPath, err := config.DefaultPath()
	if err != nil {
		return nil, err
	}

	installer, err := platform.NewDefault()
	if err != nil {
		return nil, err
	}

	logPath, err := logging.DefaultPath()
	if err != nil {
		return nil, err
	}

	return &Service{
		Store:          config.NewStore(configPath),
		Platform:       installer,
		Runner:         provider.NewRunner(provider.OSExecutor{}, logging.NewFileLogger(logPath)),
		ExecutablePath: os.Executable,
		PathValue:      func() string { return os.Getenv("PATH") },
		LogPath:        logging.DefaultPath,
	}, nil
}

func (s *Service) Set(startTime string, target string, out io.Writer) error {
	normalized, err := schedule.NormalizeStartTime(startTime)
	if err != nil {
		return err
	}

	binaryPath, err := s.ExecutablePath()
	if err != nil {
		return err
	}

	times, err := schedule.TimesForDay(normalized)
	if err != nil {
		return err
	}

	cfg := config.Config{
		StartTime:       normalized,
		Target:          target,
		IntervalMinutes: schedule.IntervalMinutes,
		Timezone:        "Local",
		BinaryPath:      binaryPath,
	}

	if err := s.Store.Save(cfg); err != nil {
		return err
	}

	if err := s.Platform.Install(binaryPath, s.currentPathValue(), times); err != nil {
		return err
	}

	return printPlan(out, cfg, "installed", times)
}

func (s *Service) Delete(out io.Writer) error {
	if err := s.Platform.Remove(); err != nil {
		return err
	}

	if err := s.Store.Delete(); err != nil {
		return err
	}

	_, err := fmt.Fprintln(out, "deleted schedule")
	return err
}

func (s *Service) Plan(out io.Writer) error {
	cfg, err := s.Store.Load()
	if err != nil {
		return err
	}

	times, err := schedule.TimesForDay(cfg.StartTime)
	if err != nil {
		return err
	}

	status, err := s.Platform.Status()
	if err != nil {
		return err
	}

	return printPlan(out, cfg, status, times)
}

func (s *Service) Trigger(target string, out io.Writer) error {
	cfg, err := s.Store.Load()
	if err != nil && !errors.Is(err, config.ErrNotFound) {
		return err
	}

	if target == "" {
		if err != nil {
			return config.ErrNotFound
		}
		target = cfg.Target
	}

	results, err := s.Runner.Run(context.Background(), target, "manual")
	if err != nil {
		return err
	}

	for _, result := range results {
		if _, err := fmt.Fprintf(out, "%s:\n", result.Provider); err != nil {
			return err
		}

		reply := strings.TrimSpace(result.Reply)
		if reply == "" {
			reply = "(no reply text)"
		}

		if _, err := fmt.Fprintln(out, reply); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) RunScheduled(out io.Writer) error {
	cfg, err := s.Store.Load()
	if err != nil {
		return err
	}

	_, err = s.Runner.Run(context.Background(), cfg.Target, "scheduled")
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(out, "scheduled run complete for %s\n", cfg.Target)
	return err
}

func (s *Service) Doctor(out io.Writer) error {
	configExists, err := s.Store.Exists()
	if err != nil {
		return err
	}

	status, err := s.Platform.Status()
	if err != nil {
		return err
	}

	codexStatus := "ok"
	if err := s.Runner.Available("codex"); err != nil {
		codexStatus = "missing"
	}

	claudeStatus := "ok"
	if err := s.Runner.Available("claude"); err != nil {
		claudeStatus = "missing"
	}

	_, err = fmt.Fprintf(out,
		"config: %s\nconfig path: %s\njob: %s\ncodex: %s\nclaude: %s\n",
		boolLabel(configExists),
		s.Store.Path(),
		status,
		codexStatus,
		claudeStatus,
	)
	return err
}

func (s *Service) Logs(lines int, out io.Writer) error {
	logPath, err := s.currentLogPath()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("autofresh log not found")
		}
		return err
	}

	content := strings.TrimRight(string(data), "\n")
	if content == "" {
		return nil
	}

	parts := strings.Split(content, "\n")
	if lines > len(parts) {
		lines = len(parts)
	}
	start := len(parts) - lines
	_, err = fmt.Fprintln(out, strings.Join(parts[start:], "\n"))
	return err
}

func printPlan(out io.Writer, cfg config.Config, jobStatus string, times []schedule.TimeOfDay) error {
	if _, err := fmt.Fprintf(out, "start time: %s\ntarget: %s\ntoday:\n", cfg.StartTime, cfg.Target); err != nil {
		return err
	}

	for _, slot := range times {
		if _, err := fmt.Fprintf(out, "  - %s %s\n", slot.String(), describeTarget(cfg.Target)); err != nil {
			return err
		}
	}

	_, err := fmt.Fprintf(out, "job: %s\n", jobStatus)
	return err
}

func describeTarget(target string) string {
	switch target {
	case "codex":
		return "codex"
	case "claude":
		return "claude"
	default:
		return "codex, claude"
	}
}

func boolLabel(value bool) string {
	if value {
		return "present"
	}
	return "missing"
}

func (s *Service) currentPathValue() string {
	if s.PathValue == nil {
		return os.Getenv("PATH")
	}
	return s.PathValue()
}

func (s *Service) currentLogPath() (string, error) {
	if s.LogPath != nil {
		return s.LogPath()
	}

	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(root, "autofresh", "autofresh.log"), nil
}
