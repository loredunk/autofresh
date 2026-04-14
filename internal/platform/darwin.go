package platform

import (
	"errors"
	"os"
	"path/filepath"

	"autofresh/internal/schedule"
)

type Darwin struct {
	HomeDir string
	Runner  CommandRunner
}

func (d Darwin) Install(binaryPath string, envValues map[string]string, times []schedule.TimeOfDay) error {
	path := plistPath(d.HomeDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	if err := os.WriteFile(path, []byte(BuildLaunchdPlist(binaryPath, envValues, times)), 0o644); err != nil {
		return err
	}

	if d.Runner != nil {
		_, _ = d.Runner.Run("launchctl", []string{"unload", path}, "")
		if _, err := d.Runner.Run("launchctl", []string{"load", path}, ""); err != nil {
			return err
		}
	}

	return nil
}

func (d Darwin) Remove() error {
	path := plistPath(d.HomeDir)

	if d.Runner != nil {
		_, _ = d.Runner.Run("launchctl", []string{"unload", path}, "")
	}

	err := os.Remove(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return nil
}

func (d Darwin) Status() (string, error) {
	_, err := os.Stat(plistPath(d.HomeDir))
	if err == nil {
		return "installed", nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return "not installed", nil
	}
	return "", err
}
