package job

import (
	"fmt"
	"log"
	"time"

	"github.com/NERVEbing/ikuai-aio/config"
	"github.com/go-co-op/gocron"
)

func Run(c *config.Config) error {
	if len(c.IKuaiCronCustomISPList)+len(c.IKuaiCronStreamDomainList) == 0 {
		logger("Run", "there are currently no tasks")
		return nil
	}

	cron := gocron.NewScheduler(c.Timezone)

	for id, i := range c.IKuaiCronCustomISPList {
		interval, err := time.ParseDuration(i.Cron)
		if err != nil {
			cron = cron.Cron(i.Cron)
		} else {
			cron = cron.Every(interval)
			if c.IKuaiCronSkipStart {
				cron = cron.StartAt(time.Now().Add(interval))
			}
		}

		if _, err = cron.Do(func() {
			_, next := cron.NextRun()
			logger("updateCustomISP", "id: %s, running...", id)
			err = updateCustomISP(i)
			logger("updateCustomISP", "id: %s, finished, error: %v, next run time: %s", id, err, next.String())
		}); err != nil {
			log.Println(err)
		}
		logger("updateCustomISP", "id: %s, cron/interval: %s, skip start: %t, timezone: %s", id, i.Cron, c.IKuaiCronSkipStart, c.Timezone)
	}

	for id, i := range c.IKuaiCronStreamDomainList {
		interval, err := time.ParseDuration(i.Cron)
		if err != nil {
			cron = cron.Cron(i.Cron)
		} else {
			cron = cron.Every(interval)
			if c.IKuaiCronSkipStart {
				cron = cron.StartAt(time.Now().Add(interval))
			}
		}

		if _, err = cron.Do(func() {
			_, next := cron.NextRun()
			logger("updateStreamDomain", "id: %s, running...", id)
			err = updateStreamDomain(i)
			logger("updateStreamDomain", "id: %s, finished, error: %v, next run time: %s", id, err, next.String())
		}); err != nil {
			log.Println(err)
		}
		logger("updateStreamDomain", "id: %s, cron/interval: %s, skip start: %t, timezone: %s", id, i.Cron, c.IKuaiCronSkipStart, c.Timezone)
	}

	if cron.Len() == 0 {
		logger("Run", "ikuai job length: %d, skip running", cron.Len())
		return nil
	}
	logger("Run", "ikuai job length: %d", cron.Len())
	cron.StartBlocking()

	return nil
}

func logger(tag string, format string, v ...any) {
	s := fmt.Sprintf("[job] tag: [%s], %s", tag, fmt.Sprintf(format, v...))
	log.Printf(s)
}
