package platform

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
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
	Install(binaryPath string, envValues map[string]string, times []schedule.TimeOfDay) error
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
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		return stdout.String(), nil
	}

	summary := strings.TrimSpace(stderr.String())
	if summary == "" {
		return stdout.String(), err
	}

	return stdout.String(), errors.New(summary)
}

func BuildLaunchdPlist(binaryPath string, envValues map[string]string, times []schedule.TimeOfDay) string {
	var builder strings.Builder
	merged := mergedEnvValues(envValues)
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
	for _, key := range sortedKeys(merged) {
		builder.WriteString("\t\t<key>" + key + "</key>\n")
		builder.WriteString("\t\t<string>" + merged[key] + "</string>\n")
	}
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

func BuildCronBlock(binaryPath string, envValues map[string]string, times []schedule.TimeOfDay) string {
	if binaryPath == "" || len(times) == 0 {
		return ""
	}

	merged := mergedEnvValues(envValues)
	var builder strings.Builder
	builder.WriteString(cronStart)
	builder.WriteString("\n")
	for _, key := range sortedKeys(merged) {
		builder.WriteString(key + "=" + merged[key] + "\n")
	}
	for _, slot := range times {
		builder.WriteString(fmt.Sprintf("%d %d * * * %s run >/dev/null 2>&1\n", slot.Minute, slot.Hour, binaryPath))
	}
	builder.WriteString(cronEnd)
	builder.WriteString("\n")
	return builder.String()
}

func RewriteCron(existing string, binaryPath string, envValues map[string]string, times []schedule.TimeOfDay) string {
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
	builder.WriteString(BuildCronBlock(binaryPath, envValues, times))
	return builder.String()
}

func mergedEnvValues(envValues map[string]string) map[string]string {
	out := make(map[string]string, len(envValues)+1)
	for key, value := range envValues {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out[key] = value
	}
	out["PATH"] = mergedPathValue(out["PATH"])
	return out
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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
