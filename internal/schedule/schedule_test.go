package schedule

import (
	"testing"
	"time"
)

func TestIntervalsKeepFractionalSeconds(t *testing.T) {
	now := time.Unix(1000, 123456789)
	for _, spec := range []string{"1500ms", "@every 1.5s"} {
		schedule, err := Parse(spec)
		if err != nil {
			t.Fatal(err)
		}
		if schedule.Next(now).Sub(now) != 1500*time.Millisecond {
			t.Fatalf("%s rounded down", spec)
		}
	}
}

func TestCronUsesTaskTimezone(t *testing.T) {
	schedule, err := Parse("0 6 * * *")
	if err != nil {
		t.Fatal(err)
	}
	zone := time.FixedZone("Asia/Shanghai", 8*3600)
	now := time.Date(2026, 10, 1, 5, 0, 0, 0, zone)
	if next := schedule.Next(now); next.Hour() != 6 || next.Location() != zone {
		t.Fatalf("wrong next execution: %s", next)
	}
}
