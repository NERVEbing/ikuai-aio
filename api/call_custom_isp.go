package api

import (
	"encoding/json"
	"errors"
	"net"
	"strconv"
	"strings"
)

func (c *Client) CustomISPShow() (*CustomISPShowResp, error) {
	req := &CallReq{
		FuncName: "custom_isp",
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

	var mod CustomISPShowResp
	if err = json.Unmarshal(resp, &mod); err != nil {
		return nil, err
	}
	if mod.Result != 30000 {
		return nil, errors.New(mod.ErrMsg)
	}

	return &mod, nil
}

func (c *Client) CustomISPDel(ids []int) error {
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
		FuncName: "custom_isp",
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

	var mod CustomISPDelResp
	if err = json.Unmarshal(resp, &mod); err != nil {
		return err
	}
	if mod.Result != 30000 {
		return errors.New(mod.ErrMsg)
	}

	return nil
}

func (c *Client) CustomISPAdd(name string, ipGroupSlice []string, comment string) (int, error) {
	m := map[string]bool{}
	if len(comment) == 0 {
		comment = "ikuai-aio"
	}
	for _, i := range ipGroupSlice {
		ip := net.ParseIP(i)
		if ip != nil && ip.To4() != nil {
			i = ip.String()
		} else {
			ip, cidr, err := net.ParseCIDR(i)
			if err != nil || ip.To4() == nil {
				continue
			}
			i = cidr.String()
		}
		if _, exist := m[i]; !exist {
			m[i] = false
		}
	}

	ipGroupSlice = make([]string, 0, len(m))
	for row := range m {
		ipGroupSlice = append(ipGroupSlice, row)
	}

	chunkSize := 5000
	ipGroupSlices := chunkSliceStr(ipGroupSlice, chunkSize)
	for _, slice := range ipGroupSlices {
		req := &CallReq{
			FuncName: "custom_isp",
			Action:   "add",
			Param: map[string]string{
				"name":    name,
				"ipgroup": strings.Join(slice, ","),
				"comment": comment,
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

		var mod CustomISPAddResp
		if err = json.Unmarshal(resp, &mod); err != nil {
			return 0, err
		}
		if mod.Result != 30000 {
			return 0, errors.New(mod.ErrMsg)
		}
	}

	return len(ipGroupSlice), nil
}
