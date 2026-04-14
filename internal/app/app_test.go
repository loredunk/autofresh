package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"autofresh/internal/config"
	"autofresh/internal/provider"
	"autofresh/internal/schedule"
)

type fakeStore struct {
	cfg     config.Config
	saved   bool
	deleted bool
	exists  bool
	loadErr error
	path    string
}

func (f *fakeStore) Load() (config.Config, error) {
	if f.loadErr != nil {
		return config.Config{}, f.loadErr
	}
	return f.cfg, nil
}

func (f *fakeStore) Save(cfg config.Config) error {
	f.cfg = cfg
	f.saved = true
	f.exists = true
	return nil
}

func (f *fakeStore) Delete() error {
	f.deleted = true
	f.exists = false
	return nil
}

func (f *fakeStore) Exists() (bool, error) {
	return f.exists, nil
}

func (f *fakeStore) Path() string {
	return f.path
}

type fakePlatform struct {
	installed bool
	removed   bool
	status    string
	times     []schedule.TimeOfDay
	path      string
	env       map[string]string
}

func (f *fakePlatform) Install(binaryPath string, envValues map[string]string, times []schedule.TimeOfDay) error {
	f.installed = true
	f.path = binaryPath
	f.env = envValues
	f.times = times
	return nil
}

func (f *fakePlatform) Remove() error {
	f.removed = true
	return nil
}

func (f *fakePlatform) Status() (string, error) {
	if f.status == "" {
		return "not installed", nil
	}
	return f.status, nil
}

type fakeRunner struct {
	targets    []string
	availables map[string]error
	results    []provider.ExecutionResult
}

func (f *fakeRunner) Run(_ context.Context, target string, _ string) ([]provider.ExecutionResult, error) {
	f.targets = append(f.targets, target)
	return f.results, nil
}

func (f *fakeRunner) Available(name string) error {
	return f.availables[name]
}

func TestSetPersistsConfigAndInstallsJob(t *testing.T) {
	t.Parallel()

	store := &fakeStore{path: "/tmp/config.json"}
	platform := &fakePlatform{status: "installed"}
	runner := &fakeRunner{}
	service := &Service{
		Store:    store,
		Platform: platform,
		Runner:   runner,
		ExecutablePath: func() (string, error) {
			return "/tmp/autofresh", nil
		},
		EnvValues: func() map[string]string {
			return map[string]string{
				"PATH":        "/custom/bin:/usr/bin",
				"HTTPS_PROXY": "http://proxy.local:8080",
			}
		},
	}

	var out bytes.Buffer
	if err := service.Set("08:00", "all", &out); err != nil {
		t.Fatal(err)
	}

	if !store.saved {
		t.Fatal("expected config saved")
	}

	if !platform.installed {
		t.Fatal("expected platform install")
	}

	if len(platform.times) != 4 {
		t.Fatalf("got %d times", len(platform.times))
	}

	if platform.env["PATH"] != "/custom/bin:/usr/bin" {
		t.Fatalf("got PATH %q", platform.env["PATH"])
	}

	if platform.env["HTTPS_PROXY"] != "http://proxy.local:8080" {
		t.Fatalf("got HTTPS_PROXY %q", platform.env["HTTPS_PROXY"])
	}
}

func TestPlanPrintsTodaySchedule(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		path:   "/tmp/config.json",
		exists: true,
		cfg: config.Config{
			StartTime: "08:00",
			Target:    "all",
		},
	}
	platform := &fakePlatform{status: "installed"}
	service := &Service{
		Store:    store,
		Platform: platform,
		Runner:   &fakeRunner{},
		ExecutablePath: func() (string, error) {
			return "", errors.New("unused")
		},
	}

	var out bytes.Buffer
	if err := service.Plan(&out); err != nil {
		t.Fatal(err)
	}

	text := out.String()
	for _, want := range []string{"08:00", "13:10", "18:20", "23:30"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %s in output %q", want, text)
		}
	}

	for _, want := range []string{"codex model=gpt-5.4-mini", "claude model=haiku", "prompt=\"ok\""} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %s in output %q", want, text)
		}
	}
}

func TestTriggerPrintsModelReply(t *testing.T) {
	t.Parallel()

	service := &Service{
		Store: &fakeStore{
			cfg: config.Config{
				Target: "codex",
			},
		},
		Platform: &fakePlatform{},
		Runner: &fakeRunner{
			results: []provider.ExecutionResult{{
				Provider: "codex",
				Reply:    "OK.",
			}},
		},
		ExecutablePath: func() (string, error) { return "", nil },
	}

	var out bytes.Buffer
	if err := service.Trigger("codex", &out); err != nil {
		t.Fatal(err)
	}

	text := out.String()
	if !strings.Contains(text, "codex:") || !strings.Contains(text, "OK.") {
		t.Fatalf("unexpected output %q", text)
	}
}

func TestLogsPrintsTailLines(t *testing.T) {
	t.Parallel()

	logPath := filepath.Join(t.TempDir(), "autofresh.log")
	content := "one\ntwo\nthree\n"
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	service := &Service{
		Store:          &fakeStore{},
		Platform:       &fakePlatform{},
		Runner:         &fakeRunner{},
		ExecutablePath: func() (string, error) { return "", nil },
		LogPath:        func() (string, error) { return logPath, nil },
	}

	var out bytes.Buffer
	if err := service.Logs(2, &out); err != nil {
		t.Fatal(err)
	}

	text := out.String()
	if strings.Contains(text, "one") {
		t.Fatalf("unexpected first line in %q", text)
	}
	if !strings.Contains(text, "two") || !strings.Contains(text, "three") {
		t.Fatalf("missing tail lines in %q", text)
	}
}
