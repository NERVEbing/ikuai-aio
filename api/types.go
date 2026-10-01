package api

import (
	"encoding/json"
	"errors"
	"math"
	"strconv"
)

// Number handles numeric strings emitted by some 4.x builds. Invalid
// measurements fail decoding instead of silently turning into zero.
type Number float64

func (n *Number) UnmarshalJSON(raw []byte) error {
	s := string(raw)
	if len(raw) > 0 && raw[0] == '"' {
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return errors.New("invalid numeric measurement")
	}
	*n = Number(v)
	return nil
}

type System struct {
	CPU      []string `json:"cpu"`
	CPUTemp  []Number `json:"cputemp"`
	Hostname string   `json:"hostname"`
	Uptime   Number   `json:"uptime"`
	Memory   struct {
		Total     Number `json:"total"`
		Available Number `json:"available"`
		Cached    Number `json:"cached"`
		Buffers   Number `json:"buffers"`
	} `json:"memory"`
	OnlineUser struct {
		Count Number `json:"count"`
	} `json:"online_user"`
	Stream  Traffic `json:"stream"`
	Version struct {
		Version     string `json:"version"`
		Arch        string `json:"arch"`
		Description string `json:"verstring"`
	} `json:"verinfo"`
}

type Traffic struct {
	Upload      Number `json:"upload"`
	Download    Number `json:"download"`
	TotalUp     Number `json:"total_up"`
	TotalDown   Number `json:"total_down"`
	Connections Number `json:"connect_num"`
}

type InterfaceCheck struct {
	Interface string `json:"interface"`
	Parent    string `json:"parent_interface"`
	Internet  string `json:"internet"`
	Updated   string `json:"updatetime"`
	Result    string `json:"result"`
}

type InterfaceStream struct {
	Interface   string `json:"interface"`
	Comment     string `json:"comment"`
	Address     string `json:"ip_addr"`
	Connections string `json:"connect_num"`
	Upload      Number `json:"upload"`
	Download    Number `json:"download"`
	TotalUp     Number `json:"total_up"`
	TotalDown   Number `json:"total_down"`
}

type Interfaces struct {
	Checks  []InterfaceCheck  `json:"iface_check"`
	Streams []InterfaceStream `json:"iface_stream"`
}

type OnlineClient struct {
	ID      int64  `json:"id"`
	MAC     string `json:"mac"`
	Address string `json:"ip_addr"`
	Name    string `json:"termname"`
	Comment string `json:"comment"`
	Traffic
}

type IPValue struct {
	IP      string `json:"ip"`
	Comment string `json:"comment"`
}

type IPObjectInput struct {
	Name   string    `json:"group_name"`
	Values []IPValue `json:"group_value"`
}

type IPObject struct {
	ID int64 `json:"id"`
	IPObjectInput
}

type CustomValues struct {
	Custom []string `json:"custom"`
}

type WeeklyTime struct {
	Type     string `json:"type"`
	Weekdays string `json:"weekdays"`
	Start    string `json:"start_time"`
	End      string `json:"end_time"`
}

type DomainRuleInput struct {
	Name      string       `json:"tagname"`
	Interface string       `json:"interface"`
	Enabled   string       `json:"enabled"`
	Priority  int          `json:"prio"`
	Comment   string       `json:"comment"`
	Domain    CustomValues `json:"domain"`
	Source    CustomValues `json:"src_addr"`
	Time      struct {
		Custom []WeeklyTime `json:"custom"`
	} `json:"time"`
}

// Listing resolves IDs only. Read-side object maps are intentionally separate
// from the arrays accepted by POST/PUT.
type DomainRule struct {
	ID   int64  `json:"id"`
	Name string `json:"tagname"`
}
