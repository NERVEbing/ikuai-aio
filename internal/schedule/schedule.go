package schedule

import (
	"errors"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

func Parse(expression string) (cron.Schedule, error) {
	if value, ok := strings.CutPrefix(expression, "@every "); ok {
		interval, err := time.ParseDuration(value)
		if err != nil || interval < time.Second {
			return nil, errors.New("@every requires an interval of at least 1s")
		}
		return intervalSchedule(interval), nil
	}
	if interval, err := time.ParseDuration(expression); err == nil {
		if interval < time.Second {
			return nil, errors.New("task interval must be at least 1s")
		}
		return intervalSchedule(interval), nil
	}
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	return parser.Parse(expression)
}

type intervalSchedule time.Duration

func (s intervalSchedule) Next(now time.Time) time.Time { return now.Add(time.Duration(s)) }
