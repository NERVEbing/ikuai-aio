package job

import (
	"context"
	"time"

	"github.com/NERVEbing/ikuai-aio/config"
	ikuaiapi "github.com/zy84338719/ikuai-api"
	"github.com/zy84338719/ikuai-api/service"
)

func updateStreamDomain(c *config.IKuaiCronStreamDomain, tag string) error {
	var rows []string
	start := time.Now()
	for _, url := range c.Url {
		r, err := fetch(url)
		if err != nil {
			logger(tag, "fetch %s failed, error: %s", url, err)
			continue
		}
		logger(tag, "fetch %s success, rows: %d", url, len(r))
		rows = append(rows, r...)
	}
	logger(tag, "fetch total rows: %d", len(rows))
	if len(rows) == 0 {
		return nil
	}

	conf := config.Load()
	ctx := context.Background()
	client, err := ikuaiapi.NewClientWithLoginContext(ctx, conf.IKuaiAddr, conf.IKuaiUsername, conf.IKuaiPassword,
		ikuaiapi.WithTimeout(conf.HttpTimeout),
		ikuaiapi.WithInsecureSkipVerify(conf.HttpInsecureSkipVerify),
	)
	if err != nil {
		return err
	}
	defer client.Close()

	fw := service.NewFirewallService(client)

	items, err := fw.GetStreamDomain(ctx)
	if err != nil {
		return err
	}
	var ids []int
	for _, item := range items {
		if item.Comment == c.Comment {
			ids = append(ids, item.ID)
		}
	}
	if err = fw.DelStreamDomain(ctx, ids); err != nil {
		return err
	}
	count, err := fw.AddStreamDomain(ctx, c.Interface, rows, c.SrcAddr, c.Comment)
	if err != nil {
		return err
	}
	logger(tag, "add stream domain unique rows count: %d, duration: %s", count, time.Since(start).String())

	return nil
}
