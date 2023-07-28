package main

import (
	"log"

	"github.com/NERVEbing/ikuai-aio/config"
	"github.com/NERVEbing/ikuai-aio/exporter"
	"github.com/NERVEbing/ikuai-aio/job"
	"golang.org/x/sync/errgroup"
)

func main() {
	c := config.Load()
	eg := errgroup.Group{}

	eg.Go(func() error {
		return job.Run(c)
	})
	eg.Go(func() error {
		return exporter.Run(c)
	})

	if err := eg.Wait(); err != nil {
		log.Fatalln(err)
	}
}
