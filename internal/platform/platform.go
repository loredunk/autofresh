package platform

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"autofresh/internal/schedule"
)

const (
	launchdLabel = "com.autofresh.runner"
	cronStart    = "# autofresh:start"
	cronEnd      = "# autofresh:end"
	envPath      = "/usr/local/bin:/opt/homebrew/bin:/usr/bin:/bin"
)

type Installer interface {
	Install(binaryPath string, pathValue string, times []schedule.TimeOfDay) error
	Remove() error
	Status() (string, error)
}

type CommandRunner interface {
	Run(name string, args []string, input string) (string, error)
}

type OSRunner struct{}

func NewDefault() (Installer, error) {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		return Darwin{HomeDir: home, Runner: OSRunner{}}, nil
	case "linux":
		return Linux{Runner: OSRunner{}}, nil
	default:
		return nil, fmt.Errorf("unsupported platform %s", runtime.GOOS)
	}
}

func (OSRunner) Run(name string, args []string, input string) (string, error) {
	cmd := exec.Command(name, args...)
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = io.Discard

	err := cmd.Run()
	return stdout.String(), err
}

func BuildLaunchdPlist(binaryPath string, pathValue string, times []schedule.TimeOfDay) string {
	var builder strings.Builder
	mergedPath := mergedPathValue(pathValue)
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	builder.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	builder.WriteString(`<plist version="1.0">` + "\n")
	builder.WriteString("<dict>\n")
	builder.WriteString("\t<key>Label</key>\n")
	builder.WriteString("\t<string>" + launchdLabel + "</string>\n")
	builder.WriteString("\t<key>ProgramArguments</key>\n")
	builder.WriteString("\t<array>\n")
	builder.WriteString("\t\t<string>" + binaryPath + "</string>\n")
	builder.WriteString("\t\t<string>run</string>\n")
	builder.WriteString("\t</array>\n")
	builder.WriteString("\t<key>EnvironmentVariables</key>\n")
	builder.WriteString("\t<dict>\n")
	builder.WriteString("\t\t<key>PATH</key>\n")
	builder.WriteString("\t\t<string>" + mergedPath + "</string>\n")
	builder.WriteString("\t</dict>\n")
	builder.WriteString("\t<key>StartCalendarInterval</key>\n")
	builder.WriteString("\t<array>\n")
	for _, slot := range times {
		builder.WriteString("\t\t<dict>\n")
		builder.WriteString(fmt.Sprintf("\t\t\t<key>Hour</key>\n\t\t\t<integer>%d</integer>\n", slot.Hour))
		builder.WriteString(fmt.Sprintf("\t\t\t<key>Minute</key>\n\t\t\t<integer>%d</integer>\n", slot.Minute))
		builder.WriteString("\t\t</dict>\n")
	}
	builder.WriteString("\t</array>\n")
	builder.WriteString("</dict>\n")
	builder.WriteString("</plist>\n")
	return builder.String()
}

func BuildCronBlock(binaryPath string, pathValue string, times []schedule.TimeOfDay) string {
	if binaryPath == "" || len(times) == 0 {
		return ""
	}

	mergedPath := mergedPathValue(pathValue)
	var builder strings.Builder
	builder.WriteString(cronStart)
	builder.WriteString("\n")
	builder.WriteString("PATH=" + mergedPath + "\n")
	for _, slot := range times {
		builder.WriteString(fmt.Sprintf("%d %d * * * %s run >/dev/null 2>&1\n", slot.Minute, slot.Hour, binaryPath))
	}
	builder.WriteString(cronEnd)
	builder.WriteString("\n")
	return builder.String()
}

func RewriteCron(existing string, binaryPath string, pathValue string, times []schedule.TimeOfDay) string {
	lines := strings.Split(existing, "\n")
	filtered := make([]string, 0, len(lines))
	skipping := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch trimmed {
		case cronStart:
			skipping = true
			continue
		case cronEnd:
			skipping = false
			continue
		}
		if !skipping && trimmed != "" {
			filtered = append(filtered, line)
		}
	}

	var builder strings.Builder
	for _, line := range filtered {
		builder.WriteString(line)
		builder.WriteString("\n")
	}
	builder.WriteString(BuildCronBlock(binaryPath, pathValue, times))
	return builder.String()
}

func mergedPathValue(pathValue string) string {
	pathValue = strings.TrimSpace(pathValue)
	if pathValue == "" {
		return envPath
	}

	parts := strings.Split(pathValue, ":")
	for _, fallback := range strings.Split(envPath, ":") {
		if !containsPathPart(parts, fallback) {
			parts = append(parts, fallback)
		}
	}

	return strings.Join(parts, ":")
}

func containsPathPart(parts []string, needle string) bool {
	for _, part := range parts {
		if strings.TrimSpace(part) == needle {
			return true
		}
	}
	return false
}

func plistPath(home string) string {
	return filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist")
}
