package api

import (
	"encoding/json"
	"errors"
)

func (c *Client) HomepageShowSysStat() (*HomepageShowSysStatResp, error) {
	req := &CallReq{
		FuncName: "homepage",
		Action:   "show",
		Param: map[string]string{
			"TYPE": "sysstat",
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

	var mod HomepageShowSysStatResp
	if err = json.Unmarshal(resp, &mod); err != nil {
		return nil, err
	}
	if mod.Result != 30000 {
		return nil, errors.New(mod.ErrMsg)
	}

	return &mod, nil
}
