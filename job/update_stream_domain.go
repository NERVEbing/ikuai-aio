package job

import (
	"bufio"
	"net/http"

	"github.com/NERVEbing/ikuai-aio/api"
	"github.com/NERVEbing/ikuai-aio/config"
)

func updateStreamDomain(c *config.IKuaiCronStreamDomain) error {
	var rows []string
	for _, url := range c.Url {
		r, err := fetch(url)
		if err != nil {
			logger("updateStreamDomain", "fetch %s error: %s", url, err)
			continue
		}
		logger("updateStreamDomain", "fetch %s success, rows: %d", url, len(r))
		rows = append(rows, r...)
	}
	logger("updateStreamDomain", "fetch total: %d", len(rows))
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
		ids = append(ids, i.ID)
	}
	if err = client.StreamDomainDel(ids); err != nil {
		return err
	}
	count, err := client.StreamDomainAdd(c.Interface, rows, c.SrcAddr, c.Comment)
	if err != nil {
		return err
	}
	logger("updateStreamDomain", "update stream domain success, count: %d", count)

	return nil
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
