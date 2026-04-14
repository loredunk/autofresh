package platform

import (
	"strings"

	"autofresh/internal/schedule"
)

type Linux struct {
	Runner CommandRunner
}

func (l Linux) Install(binaryPath string, pathValue string, times []schedule.TimeOfDay) error {
	current, _ := l.currentCron()
	updated := RewriteCron(current, binaryPath, pathValue, times)
	_, err := l.Runner.Run("crontab", []string{"-"}, updated)
	return err
}

func (l Linux) Remove() error {
	current, _ := l.currentCron()
	updated := RewriteCron(current, "", "", nil)
	if strings.TrimSpace(updated) == "" {
		_, err := l.Runner.Run("crontab", []string{"-r"}, "")
		return err
	}
	_, err := l.Runner.Run("crontab", []string{"-"}, updated)
	return err
}

func (l Linux) Status() (string, error) {
	current, err := l.currentCron()
	if err != nil {
		return "", err
	}
	if strings.Contains(current, cronStart) {
		return "installed", nil
	}
	return "not installed", nil
}

func (l Linux) currentCron() (string, error) {
	if l.Runner == nil {
		return "", nil
	}
	current, err := l.Runner.Run("crontab", []string{"-l"}, "")
	if err != nil {
		return "", nil
	}
	return current, nil
}
