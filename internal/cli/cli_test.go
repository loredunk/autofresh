package cli

import (
	"bytes"
	"io"
	"testing"
)

func TestParseTargetRejectsInvalidValue(t *testing.T) {
	t.Parallel()

	_, err := ParseTarget("bad")
	if err == nil {
		t.Fatal("expected error")
	}
}

type stubHandler struct {
	setStart  string
	setTarget string
	logLines  int
}

func (s *stubHandler) Set(startTime string, target string, _ io.Writer) error {
	s.setStart = startTime
	s.setTarget = target
	return nil
}

func (s *stubHandler) Delete(io.Writer) error          { return nil }
func (s *stubHandler) Plan(io.Writer) error            { return nil }
func (s *stubHandler) Trigger(string, io.Writer) error { return nil }
func (s *stubHandler) RunScheduled(io.Writer) error    { return nil }
func (s *stubHandler) Doctor(io.Writer) error          { return nil }
func (s *stubHandler) Logs(lines int, _ io.Writer) error {
	s.logLines = lines
	return nil
}

func TestRunSetAcceptsTimeBeforeTargetFlag(t *testing.T) {
	t.Parallel()

	handler := &stubHandler{}
	var out bytes.Buffer
	err := Run([]string{"set", "06:00", "--target", "all"}, Dependencies{
		App:    handler,
		Stdout: &out,
	})
	if err != nil {
		t.Fatal(err)
	}

	if handler.setStart != "06:00" {
		t.Fatalf("got start %q", handler.setStart)
	}

	if handler.setTarget != "all" {
		t.Fatalf("got target %q", handler.setTarget)
	}
}

func TestRunLogsDefaultsToTwentyLines(t *testing.T) {
	t.Parallel()

	handler := &stubHandler{}
	err := Run([]string{"logs"}, Dependencies{
		App:    handler,
		Stdout: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatal(err)
	}

	if handler.logLines != 20 {
		t.Fatalf("got log lines %d", handler.logLines)
	}
}

func TestRunLogsAcceptsNFlag(t *testing.T) {
	t.Parallel()

	handler := &stubHandler{}
	err := Run([]string{"logs", "-n", "5"}, Dependencies{
		App:    handler,
		Stdout: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatal(err)
	}

	if handler.logLines != 5 {
		t.Fatalf("got log lines %d", handler.logLines)
	}
}
