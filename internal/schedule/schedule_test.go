package schedule

import (
	"reflect"
	"testing"
)

func TestTimesForDay(t *testing.T) {
	t.Parallel()

	got, err := TimesForDay("08:00")
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"08:00", "13:10", "18:20", "23:30"}
	if !reflect.DeepEqual(FormatTimes(got), want) {
		t.Fatalf("got %v want %v", FormatTimes(got), want)
	}
}

func TestTimesForDaySingleSlot(t *testing.T) {
	t.Parallel()

	got, err := TimesForDay("21:00")
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"21:00"}
	if !reflect.DeepEqual(FormatTimes(got), want) {
		t.Fatalf("got %v want %v", FormatTimes(got), want)
	}
}
