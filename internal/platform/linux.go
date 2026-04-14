package platform

import (
	"errors"
	"strings"

	"autofresh/internal/schedule"
)

type Linux struct {
	Runner CommandRunner
}

func (l Linux) Install(binaryPath string, envValues map[string]string, times []schedule.TimeOfDay) error {
	current, _ := l.currentCron()
	updated := RewriteCron(current, binaryPath, envValues, times)
	_, err := l.Runner.Run("crontab", []string{"-"}, updated)
	return err
}

func (l Linux) Remove() error {
	current, _ := l.currentCron()
	updated := RewriteCron(current, "", nil, nil)
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
		return "unknown: " + strings.TrimSpace(err.Error()), nil
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
		message := strings.ToLower(err.Error())
		if strings.Contains(message, "no crontab") {
			return "", nil
		}
		return "", errors.New(strings.TrimSpace(err.Error()))
	}
	return current, nil
}
