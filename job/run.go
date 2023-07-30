package job

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/NERVEbing/ikuai-aio/config"
	"github.com/go-co-op/gocron"
)

func Run(c *config.Config) error {
	cron := gocron.NewScheduler(c.Timezone)
	cron.SetMaxConcurrentJobs(1, gocron.WaitMode)

	for n, i := range c.IKuaiCronCustomISPList {
		tag := "updateCustomISP" + "-" + strconv.Itoa(n+1)
		cron = setCron(cron, i.Cron, c.IKuaiCronSkipStart).Name(tag).Tag(tag)
		if _, err := cron.Do(updateCustomISP, i, tag); err != nil {
			logger("cron", "tag: %s, error: %v", tag, err)
		}
		logger(tag, "cron/interval: %s, skip start: %t, timezone: %s", i.Cron, c.IKuaiCronSkipStart, c.Timezone)
	}

	for n, i := range c.IKuaiCronStreamDomainList {
		tag := "updateStreamDomain" + "-" + strconv.Itoa(n+1)
		cron = setCron(cron, i.Cron, c.IKuaiCronSkipStart).Name(tag).Tag(tag)
		if _, err := cron.Do(updateStreamDomain, i, tag); err != nil {
			logger("cron", "tag: %s, error: %v", tag, err)
		}
		logger(tag, "cron/interval: %s, skip start: %t, timezone: %s", i.Cron, c.IKuaiCronSkipStart, c.Timezone)
	}

	cron.RegisterEventListeners(
		gocron.BeforeJobRuns(func(tag string) {
			logger(tag, "running...")
		}),
		gocron.AfterJobRuns(func(tag string) {
			jobs, err := cron.FindJobsByTag(tag)
			if err != nil {
				logger("cron", "error: %s", err.Error())
			}
			for _, i := range jobs {
				logger(tag, "finished, next run time: %s", i.NextRun().String())
			}
		}),
		gocron.WhenJobReturnsNoError(func(tag string) {
			logger(tag, "success")
		}),
		gocron.WhenJobReturnsError(func(tag string, err error) {
			logger(tag, "failed, error: %s", err.Error())
		}),
	)

	logger("Run", "job length: %d", cron.Len())
	cron.StartBlocking()

	return nil
}

func logger(tag string, format string, v ...any) {
	s := fmt.Sprintf("[job] tag: [%s], %s", tag, fmt.Sprintf(format, v...))
	log.Printf(s)
}

func fetch(url string) ([]string, error) {
	var rows []string
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(resp.Body)
	defer func() {
		if err = resp.Body.Close(); err != nil {
			logger("defer fetch", "close body error: %s", err)
		}
	}()
	for scanner.Scan() {
		rows = append(rows, scanner.Text())
	}

	return rows, nil
}

func setCron(scheduler *gocron.Scheduler, cronStr string, isSkip bool) *gocron.Scheduler {
	interval, err := time.ParseDuration(cronStr)
	if err != nil {
		scheduler = scheduler.Cron(cronStr)
	} else {
		scheduler = scheduler.Every(interval)
		if isSkip {
			scheduler = scheduler.StartAt(time.Now().Add(interval))
		}
	}

	return scheduler
}
