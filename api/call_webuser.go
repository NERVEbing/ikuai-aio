package api

import (
	"encoding/json"
)

func (c *Client) WebUserShow() (*WebUserShowResp, error) {
	req := &CallReq{
		FuncName: "webuser",
		Action:   "show",
		Param: map[string]string{
			"TYPE": "mod_passwd",
		},
	}
	b, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	resp, err := c.request(iKuaiCallPath, b)
	if err != nil {
		return nil, err
	}

	var mod WebUserShowResp
	if err = json.Unmarshal(resp, &mod); err != nil {
		return nil, err
	}

	return &mod, nil
}
