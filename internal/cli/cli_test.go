package cli

import (
	"bytes"
	"io"
	"testing"

	"ccoach/internal/codexreport"
)

func stub() (*codexreport.Options, ReportFunc) {
	var got codexreport.Options
	fn := func(opts codexreport.Options, _ io.Writer) error {
		got = opts
		return nil
	}
	return &got, fn
}

func TestReportIsDefaultCommand(t *testing.T) {
	t.Parallel()

	got, fn := stub()
	err := Run([]string{"--json", "--days", "1"}, Dependencies{
		Report: fn,
		Stdout: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got.JSON || got.Days != 1 {
		t.Fatalf("unexpected opts: %+v", *got)
	}
}

func TestLeadingReportTokenAccepted(t *testing.T) {
	t.Parallel()

	got, fn := stub()
	err := Run([]string{"report", "--since", "2026-05-01"}, Dependencies{
		Report: fn,
		Stdout: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Since != "2026-05-01" {
		t.Fatalf("unexpected opts: %+v", *got)
	}
}

func TestRejectsMultipleWindows(t *testing.T) {
	t.Parallel()

	_, fn := stub()
	err := Run([]string{"--days", "7", "--date", "2026-05-01"}, Dependencies{
		Report: fn,
		Stdout: &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("expected error for conflicting window flags")
	}
}

func TestRejectsPositionalArgs(t *testing.T) {
	t.Parallel()

	_, fn := stub()
	err := Run([]string{"oops"}, Dependencies{
		Report: fn,
		Stdout: &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("expected error for positional argument")
	}
}
