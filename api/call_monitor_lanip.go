package api

import (
	"encoding/json"
	"errors"
)

func (c *Client) MonitorLanIPShow() (*MonitorLanIPShowResp, error) {
	req := &CallReq{
		FuncName: "monitor_lanip",
		Action:   "show",
		Param: map[string]string{
			"TYPE": "data",
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

	var mod MonitorLanIPShowResp
	if err = json.Unmarshal(resp, &mod); err != nil {
		return nil, err
	}
	if mod.Result != 30000 {
		return nil, errors.New(mod.ErrMsg)
	}

	return &mod, nil
}
