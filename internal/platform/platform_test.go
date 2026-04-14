package platform

import (
	"strings"
	"testing"

	"autofresh/internal/schedule"
)

func TestLaunchdPlistContainsAllTimes(t *testing.T) {
	t.Parallel()

	plist := BuildLaunchdPlist("/tmp/autofresh", "/custom/bin:/usr/bin", []schedule.TimeOfDay{
		{Hour: 8, Minute: 0},
		{Hour: 13, Minute: 10},
	})

	if !strings.Contains(plist, "StartCalendarInterval") {
		t.Fatal("missing intervals")
	}

	if !strings.Contains(plist, "<integer>13</integer>") {
		t.Fatal("missing hour")
	}

	if !strings.Contains(plist, "/custom/bin:/usr/bin") {
		t.Fatal("missing custom path")
	}
}

func TestCronRewritePreservesForeignEntries(t *testing.T) {
	t.Parallel()

	input := "MAILTO=test\n0 1 * * * /bin/echo hi\n"
	out := RewriteCron(input, "/tmp/autofresh", "/custom/bin:/usr/bin", []schedule.TimeOfDay{{Hour: 8, Minute: 0}})

	if !strings.Contains(out, "MAILTO=test") {
		t.Fatal("dropped foreign entry")
	}

	if !strings.Contains(out, cronStart) {
		t.Fatal("missing autofresh block")
	}

	if !strings.Contains(out, "PATH=/custom/bin:/usr/bin") {
		t.Fatal("missing custom path")
	}
}

func TestCronRewriteRemovesAutofreshBlockWhenNoTimes(t *testing.T) {
	t.Parallel()

	input := "MAILTO=test\n# autofresh:start\nPATH=/usr/bin\n0 8 * * * /tmp/autofresh run >/dev/null 2>&1\n# autofresh:end\n"
	out := RewriteCron(input, "", "", nil)

	if strings.Contains(out, cronStart) {
		t.Fatal("expected autofresh block removed")
	}

	if !strings.Contains(out, "MAILTO=test") {
		t.Fatal("expected foreign entry preserved")
	}
}
