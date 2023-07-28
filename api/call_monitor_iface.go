package api

import (
	"encoding/json"
	"errors"
)

func (c *Client) MonitorIFaceShow() (*MonitorIFaceShowResp, error) {
	req := &CallReq{
		FuncName: "monitor_iface",
		Action:   "show",
		Param: map[string]string{
			"TYPE": "iface_check,iface_stream",
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

	var mod MonitorIFaceShowResp
	if err = json.Unmarshal(resp, &mod); err != nil {
		return nil, err
	}
	if mod.Result != 30000 {
		return nil, errors.New(mod.ErrMsg)
	}

	return &mod, nil
}
