package api

import (
	"encoding/json"
	"errors"
)

func (c *Client) AuthCodeShow() error {
	req := &CallReq{
		FuncName: "auth_code",
		Action:   "show",
	}
	b, err := json.Marshal(req)
	if err != nil {
		return err
	}
	resp, err := c.request(iKuaiCallPath, b)
	if err != nil {
		return err
	}

	var mod AuthCodeShowResp
	if err = json.Unmarshal(resp, &mod); err != nil {
		return err
	}
	if mod.Result != 30000 {
		return errors.New(mod.ErrMsg)
	}

	return nil
}
