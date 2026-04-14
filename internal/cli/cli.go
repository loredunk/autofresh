package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
)

type Handler interface {
	Set(startTime string, target string, out io.Writer) error
	Delete(out io.Writer) error
	Plan(out io.Writer) error
	Trigger(target string, out io.Writer) error
	RunScheduled(out io.Writer) error
	Doctor(out io.Writer) error
	Logs(lines int, out io.Writer) error
}

type Dependencies struct {
	App    Handler
	Stdout io.Writer
	Stderr io.Writer
}

func Run(args []string, deps Dependencies) error {
	if deps.App == nil {
		return errors.New("missing app dependency")
	}

	if deps.Stdout == nil {
		deps.Stdout = io.Discard
	}

	if len(args) == 0 {
		return usageError()
	}

	switch args[0] {
	case "set":
		return runSet(args[1:], deps)
	case "delete":
		if len(args) != 1 {
			return fmt.Errorf("delete takes no arguments")
		}
		return deps.App.Delete(deps.Stdout)
	case "plan":
		if len(args) != 1 {
			return fmt.Errorf("plan takes no arguments")
		}
		return deps.App.Plan(deps.Stdout)
	case "trigger":
		return runTrigger(args[1:], deps)
	case "run":
		if len(args) != 1 {
			return fmt.Errorf("run takes no arguments")
		}
		return deps.App.RunScheduled(deps.Stdout)
	case "doctor":
		if len(args) != 1 {
			return fmt.Errorf("doctor takes no arguments")
		}
		return deps.App.Doctor(deps.Stdout)
	case "logs":
		return runLogs(args[1:], deps)
	default:
		return usageError()
	}
}

func ParseTarget(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "codex", "claude", "all":
		return normalized, nil
	default:
		return "", fmt.Errorf("invalid target %q", value)
	}
}

func runSet(args []string, deps Dependencies) error {
	startTime, flagArgs, err := splitSetArgs(args)
	if err != nil {
		return err
	}

	fs := flag.NewFlagSet("set", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	target := fs.String("target", "all", "target provider")
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}

	parsedTarget, err := ParseTarget(*target)
	if err != nil {
		return err
	}

	if len(fs.Args()) != 0 {
		return errors.New("set accepts exactly one start time like 08:00")
	}

	return deps.App.Set(startTime, parsedTarget, deps.Stdout)
}

func runTrigger(args []string, deps Dependencies) error {
	fs := flag.NewFlagSet("trigger", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	target := fs.String("target", "", "target provider")
	if err := fs.Parse(args); err != nil {
		return err
	}

	parsedTarget := ""
	if *target != "" {
		var err error
		parsedTarget, err = ParseTarget(*target)
		if err != nil {
			return err
		}
	}

	if len(fs.Args()) != 0 {
		return errors.New("trigger takes no positional arguments")
	}

	return deps.App.Trigger(parsedTarget, deps.Stdout)
}

func usageError() error {
	return errors.New("usage: autofresh <set|plan|trigger|delete|run|doctor|logs>")
}

func splitSetArgs(args []string) (string, []string, error) {
	startTime := ""
	flagArgs := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "" {
			continue
		}

		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
			if arg == "--target" {
				if i+1 >= len(args) {
					return "", nil, errors.New("flag needs an argument: -target")
				}
				i++
				flagArgs = append(flagArgs, args[i])
			}
			continue
		}

		if startTime != "" {
			return "", nil, errors.New("set accepts exactly one start time like 08:00")
		}
		startTime = arg
	}

	if startTime == "" {
		return "", nil, errors.New("set requires a start time like 08:00")
	}

	return startTime, flagArgs, nil
}

func runLogs(args []string, deps Dependencies) error {
	fs := flag.NewFlagSet("logs", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	lines := fs.Int("n", 20, "number of log lines")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if len(fs.Args()) != 0 {
		return errors.New("logs takes no positional arguments")
	}
	if *lines <= 0 {
		return errors.New("logs requires -n to be greater than 0")
	}

	return deps.App.Logs(*lines, deps.Stdout)
}
