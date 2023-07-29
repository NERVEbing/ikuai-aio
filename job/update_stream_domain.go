package job

import (
	"github.com/NERVEbing/ikuai-aio/api"
	"github.com/NERVEbing/ikuai-aio/config"
)

func updateStreamDomain(c *config.IKuaiCronStreamDomain, tag string) error {
	var rows []string
	for _, url := range c.Url {
		r, err := fetch(url)
		if err != nil {
			logger(tag, "fetch %s error: %s", url, err)
			continue
		}
		logger(tag, "fetch %s success, rows: %d", url, len(r))
		rows = append(rows, r...)
	}
	logger(tag, "fetch total: %d", len(rows))
	if len(rows) == 0 {
		return nil
	}

	client := api.NewClient()
	if err := client.Login(); err != nil {
		return err
	}
	StreamDomainResp, err := client.StreamDomainShow()
	if err != nil {
		return err
	}
	var ids []int
	for _, i := range StreamDomainResp.Data.Data {
		if i.Comment == c.Comment {
			ids = append(ids, i.ID)
		}
	}
	if err = client.StreamDomainDel(ids); err != nil {
		return err
	}
	count, err := client.StreamDomainAdd(c.Interface, rows, c.SrcAddr, c.Comment)
	if err != nil {
		return err
	}
	logger(tag, "update stream domain success, count: %d", count)

	return nil
}
