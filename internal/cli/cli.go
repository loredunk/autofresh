package cli

import (
	"errors"
	"flag"
	"io"

	"ccoach/internal/codexreport"
)

// ReportFunc renders a usage report for the given options. It is injectable so
// the CLI layer can be tested without touching the filesystem.
type ReportFunc func(opts codexreport.Options, out io.Writer) error

type Dependencies struct {
	Report ReportFunc
	Stdout io.Writer
	Stderr io.Writer
}

// Run is the ccoach entrypoint. The usage report is the default command:
//
//	ccoach [--json --days N --since YYYY-MM-DD --date YYYY-MM-DD --by-repo]
//
// A leading "report" token is still accepted for familiarity
// (`ccoach report ...`), but is optional.
func Run(args []string, deps Dependencies) error {
	if deps.Report == nil {
		deps.Report = codexreport.Run
	}
	if deps.Stdout == nil {
		deps.Stdout = io.Discard
	}

	// Accept an optional leading "report" subcommand for back-compat.
	if len(args) > 0 && args[0] == "report" {
		args = args[1:]
	}

	return runReport(args, deps)
}

func runReport(args []string, deps Dependencies) error {
	fs := flag.NewFlagSet("ccoach", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	date := fs.String("date", "", "single local day, YYYY-MM-DD")
	since := fs.String("since", "", "from this local day through today, YYYY-MM-DD")
	days := fs.Int("days", 0, "last N days including today")
	asJSON := fs.Bool("json", false, "emit JSON instead of text")
	byRepo := fs.Bool("by-repo", false, "detailed per-repository breakdown")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return errors.New("ccoach takes no positional arguments")
	}
	if *days < 0 {
		return errors.New("ccoach requires --days to be >= 0")
	}
	set := 0
	if *date != "" {
		set++
	}
	if *since != "" {
		set++
	}
	if *days > 0 {
		set++
	}
	if set > 1 {
		return errors.New("ccoach accepts only one of --date, --since, --days")
	}

	return deps.Report(codexreport.Options{
		Date:   *date,
		Since:  *since,
		Days:   *days,
		JSON:   *asJSON,
		ByRepo: *byRepo,
	}, deps.Stdout)
}
