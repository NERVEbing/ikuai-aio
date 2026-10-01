package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func (c *Client) System(ctx context.Context) (*System, error) {
	var data struct {
		Info json.RawMessage `json:"sysinfo"`
	}
	if err := c.get(ctx, "/monitoring/system", nil, &data); err != nil {
		return nil, err
	}
	fields, err := requireFields(data.Info, "cpu", "memory", "stream", "uptime", "online_user", "verinfo")
	if err != nil {
		return nil, fmt.Errorf("iKuai sysinfo: %w", err)
	}
	for key, required := range map[string][]string{
		"memory":      {"total", "available", "cached", "buffers"},
		"stream":      {"upload", "download", "total_up", "total_down", "connect_num"},
		"online_user": {"count"}, "verinfo": {"version"},
	} {
		if _, err = requireFields(fields[key], required...); err != nil {
			return nil, fmt.Errorf("iKuai sysinfo.%s: %w", key, err)
		}
	}
	var info System
	if err = json.Unmarshal(data.Info, &info); err != nil {
		return nil, fmt.Errorf("iKuai sysinfo: %w", err)
	}
	if !strings.HasPrefix(info.Version.Version, "4.") {
		return nil, errors.New("only iKuai 4.x is supported; unexpected firmware version")
	}
	return &info, nil
}

func requireFields(raw json.RawMessage, names ...string) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, errors.New("missing or invalid object")
	}
	for _, name := range names {
		value, present := fields[name]
		if !present || string(value) == "null" {
			return nil, fmt.Errorf("missing %s", name)
		}
	}
	return fields, nil
}

func (c *Client) Interfaces(ctx context.Context) (*Interfaces, error) {
	var data map[string]json.RawMessage
	if err := c.get(ctx, "/monitoring/interfaces-status", nil, &data); err != nil {
		return nil, err
	}
	checks, cOK := data["iface_check"]
	streams, sOK := data["iface_stream"]
	if !cOK || !sOK {
		return nil, errors.New("iKuai interface response is missing status or traffic")
	}
	var output Interfaces
	if err := json.Unmarshal(checks, &output.Checks); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(streams, &output.Streams); err != nil {
		return nil, err
	}
	return &output, nil
}

func (c *Client) OnlineClients(ctx context.Context, ipv6 bool) ([]OnlineClient, error) {
	endpoint := "/monitoring/clients-online"
	if ipv6 {
		endpoint = "/monitoring/clients-ip6-online"
	}
	return list[OnlineClient](ctx, c, endpoint, "data", "total")
}
