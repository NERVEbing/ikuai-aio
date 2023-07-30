package api

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

func (c *Client) StreamDomainShow() (*StreamDomainShowResp, error) {
	req := &CallReq{
		FuncName: "stream_domain",
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

	var mod StreamDomainShowResp
	if err = json.Unmarshal(resp, &mod); err != nil {
		return nil, err
	}
	if mod.Result != 30000 {
		return nil, errors.New(mod.ErrMsg)
	}

	return &mod, nil
}

func (c *Client) StreamDomainDel(ids []int) error {
	id := ""
	if len(ids) == 0 {
		return nil
	}
	var idStr []string
	for _, i := range ids {
		idStr = append(idStr, strconv.Itoa(i))
	}
	id = strings.Join(idStr, ",")
	req := &CallReq{
		FuncName: "stream_domain",
		Action:   "del",
		Param: map[string]string{
			"id": id,
		},
	}
	b, err := json.Marshal(req)
	if err != nil {
		return err
	}
	resp, err := c.request(iKuaiCallPath, b)
	if err != nil {
		return err
	}

	var mod StreamDomainDelResp
	if err = json.Unmarshal(resp, &mod); err != nil {
		return err
	}
	if mod.Result != 30000 {
		return errors.New(mod.ErrMsg)
	}

	return nil
}

func (c *Client) StreamDomainAdd(interfaceSlice []string, domainSlice []string, srcAddr string, comment string) (int, error) {
	m := map[string]bool{}
	for _, i := range domainSlice {
		if !isValidDomain(i) {
			continue
		}
		if _, exist := m[i]; !exist {
			m[i] = false
		}
	}

	domainSlice = make([]string, 0, len(m))
	for row := range m {
		domainSlice = append(domainSlice, row)
	}

	chunkSize := 1000
	domainSlices := chunkSliceStr(domainSlice, chunkSize)
	for _, slice := range domainSlices {
		req := &CallReq{
			FuncName: "stream_domain",
			Action:   "add",
			Param: map[string]string{
				"interface": strings.Join(interfaceSlice, ","),
				"src_addr":  srcAddr,
				"domain":    strings.Join(slice, ","),
				"comment":   comment,
				"week":      "1234567",
				"time":      "00:00-23:59",
				"enabled":   "yes",
			},
		}
		b, err := json.Marshal(req)
		if err != nil {
			return 0, err
		}
		resp, err := c.request(iKuaiCallPath, b)
		if err != nil {
			return 0, err
		}

		var mod StreamDomainAddResp
		if err = json.Unmarshal(resp, &mod); err != nil {
			return 0, err
		}
		if mod.Result != 30000 {
			return 0, errors.New(mod.ErrMsg)
		}
	}

	return len(domainSlice), nil
}
