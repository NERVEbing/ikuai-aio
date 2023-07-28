package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
)

var UnauthorizedError = errors.New("no login authentication")

func (c *Client) Login() error {
	req := &LoginReq{
		Username: c.iKuaiUsername,
		Password: toMD5(c.iKuaiPassword),
		Pass:     base64.StdEncoding.EncodeToString([]byte("salt_11" + c.iKuaiPassword)),
	}
	b, err := json.Marshal(req)
	if err != nil {
		return err
	}
	resp, err := c.request(iKuaiLoginPath, b)
	if err != nil {
		return err
	}

	var mod LoginResp
	if err = json.Unmarshal(resp, &mod); err != nil {
		return err
	}
	if mod.Result != 10000 {
		return errors.New(mod.ErrMsg)
	}

	return nil
}
