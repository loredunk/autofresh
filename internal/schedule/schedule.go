package schedule

import (
	"fmt"
	"strconv"
	"strings"
)

const IntervalMinutes = 310

type TimeOfDay struct {
	Hour   int
	Minute int
}

func (t TimeOfDay) String() string {
	return fmt.Sprintf("%02d:%02d", t.Hour, t.Minute)
}

func ParseTimeOfDay(value string) (TimeOfDay, error) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return TimeOfDay{}, fmt.Errorf("invalid time %q", value)
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return TimeOfDay{}, fmt.Errorf("invalid hour %q", value)
	}

	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return TimeOfDay{}, fmt.Errorf("invalid minute %q", value)
	}

	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return TimeOfDay{}, fmt.Errorf("invalid time %q", value)
	}

	return TimeOfDay{Hour: hour, Minute: minute}, nil
}

func NormalizeStartTime(value string) (string, error) {
	tod, err := ParseTimeOfDay(value)
	if err != nil {
		return "", err
	}
	return tod.String(), nil
}

func TimesForDay(start string) ([]TimeOfDay, error) {
	startTime, err := ParseTimeOfDay(start)
	if err != nil {
		return nil, err
	}

	current := startTime.Hour*60 + startTime.Minute
	var times []TimeOfDay
	for current < 24*60 {
		times = append(times, TimeOfDay{
			Hour:   current / 60,
			Minute: current % 60,
		})
		current += IntervalMinutes
	}

	return times, nil
}

func FormatTimes(times []TimeOfDay) []string {
	out := make([]string, 0, len(times))
	for _, slot := range times {
		out = append(out, slot.String())
	}
	return out
}
