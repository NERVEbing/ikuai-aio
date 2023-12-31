package api

import (
	"bytes"
	"crypto/tls"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"

	"github.com/NERVEbing/ikuai-aio/config"
)

type Client struct {
	iKuaiAddr     string
	iKuaiUsername string
	iKuaiPassword string
	Http          *http.Client
}

func NewClient() *Client {
	conf := config.Load()
	cookie, err := cookiejar.New(nil)
	if err != nil {
		log.Fatalln(err)
	}
	return &Client{
		iKuaiAddr:     conf.IKuaiAddr,
		iKuaiUsername: conf.IKuaiUsername,
		iKuaiPassword: conf.IKuaiPassword,
		Http: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: conf.HttpInsecureSkipVerify,
				},
			},
			Jar:     cookie,
			Timeout: conf.HttpTimeout,
		},
	}
}

func (c *Client) request(path string, body []byte) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, c.iKuaiAddr+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Http.Do(req)
	if err != nil {
		return nil, err
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err = resp.Body.Close(); err != nil {
			log.Println(err)
		}
	}()

	return b, nil
}
