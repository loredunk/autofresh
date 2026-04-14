package platform

import (
	"errors"
	"strings"
	"testing"
)

type fakeCommandRunner struct {
	output string
	err    error
}

func (f fakeCommandRunner) Run(_ string, _ []string, _ string) (string, error) {
	return f.output, f.err
}

func TestLinuxStatusReportsUnknownOnCrontabError(t *testing.T) {
	t.Parallel()

	linux := Linux{
		Runner: fakeCommandRunner{err: errors.New("crontabs/gavin/: fopen: 权限不够")},
	}

	status, err := linux.Status()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if !strings.HasPrefix(status, "unknown") || !strings.Contains(status, "权限不够") {
		t.Fatalf("got status %q", status)
	}
}

func TestLinuxStatusAllowsMissingCrontab(t *testing.T) {
	t.Parallel()

	linux := Linux{
		Runner: fakeCommandRunner{err: errors.New("no crontab for gavin")},
	}

	status, err := linux.Status()
	if err != nil {
		t.Fatal(err)
	}

	if status != "not installed" {
		t.Fatalf("got status %q", status)
	}
}
