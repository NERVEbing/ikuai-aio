package api

type LoginReq struct {
	Username string `json:"username"`
	Password string `json:"passwd"`
	Pass     string `json:"pass"`
}

type LoginResp struct {
	ErrMsg string `json:"ErrMsg"`
	Result int    `json:"Result"`
}

type CallReq struct {
	FuncName string      `json:"func_name"`
	Action   string      `json:"action"`
	Param    interface{} `json:"param"`
}

type CallResp struct {
	ErrMsg string `json:"ErrMsg"`
	Result int    `json:"Result"`
}

type CustomISPShowResp struct {
	CallResp
	Data struct {
		Total int `json:"total"`
		Data  []struct {
			ID      int    `json:"id"`
			Name    string `json:"name"`
			IPGroup string `json:"ipgroup"`
			Comment string `json:"comment"`
			Time    string `json:"time"`
		} `json:"data"`
	} `json:"Data"`
}

type CustomISPDelResp struct {
	CallResp
}

type CustomISPAddResp struct {
	CallResp
}

type StreamDomainShowResp struct {
	CallResp
	Data struct {
		Total int `json:"total"`
		Data  []struct {
			ID        int    `json:"id"`
			Interface string `json:"interface"`
			SrcAddr   string `json:"src_addr"`
			Enabled   string `json:"enabled"`
			Week      string `json:"week"`
			Comment   string `json:"comment"`
			Domain    string `json:"domain"`
			Time      string `json:"time"`
		} `json:"data"`
	} `json:"Data"`
}

type StreamDomainDelResp struct {
	CallResp
}

type StreamDomainAddResp struct {
	CallResp
}

type HomepageShowSysStatResp struct {
	CallResp
	Data struct {
		SysStat struct {
			Cpu        []string `json:"cpu"`
			CpuTemp    []int    `json:"cputemp"`
			Freq       []string `json:"freq"`
			GWid       string   `json:"gwid"`
			Hostname   string   `json:"hostname"`
			LinkStatus int      `json:"link_status"`
			Memory     struct {
				Total     int64  `json:"total"`
				Available int64  `json:"available"`
				Free      int64  `json:"free"`
				Cached    int64  `json:"cached"`
				Buffers   int64  `json:"buffers"`
				Used      string `json:"used"`
			} `json:"memory"`
			OnlineUser struct {
				Count         int `json:"count"`
				Count2G       int `json:"count_2g"`
				Count5G       int `json:"count_5g"`
				CountWired    int `json:"count_wired"`
				CountWireless int `json:"count_wireless"`
			} `json:"online_user"`
			Stream struct {
				ConnectNum int   `json:"connect_num"`
				Upload     int   `json:"upload"`
				Download   int   `json:"download"`
				TotalUp    int64 `json:"total_up"`
				TotalDown  int64 `json:"total_down"`
			} `json:"stream"`
			Uptime  int `json:"uptime"`
			VerInfo struct {
				ModelName    string `json:"modelname"`
				VerString    string `json:"verstring"`
				Version      string `json:"version"`
				BuildDate    int64  `json:"build_date"`
				Arch         string `json:"arch"`
				SysBit       string `json:"sysbit"`
				VerFlags     string `json:"verflags"`
				IsEnterprise int    `json:"is_enterprise"`
				SupportI18N  int    `json:"support_i18n"`
				SupportLcd   int    `json:"support_lcd"`
			} `json:"verinfo"`
		} `json:"sysstat"`
		AcStatus struct {
			ApCount  int `json:"ap_count"`
			ApOnline int `json:"ap_online"`
		} `json:"ac_status"`
	} `json:"Data"`
}

type MonitorLanIPShowResp struct {
	CallResp
	Data struct {
		Data []struct {
			ApName       string `json:"apname"`
			AcGid        int    `json:"ac_gid"`
			Mac          string `json:"mac"`
			LinkAddr     string `json:"link_addr"`
			Hostname     string `json:"hostname"`
			DTalkName    string `json:"dtalk_name"`
			DownRate     string `json:"downrate"`
			Reject       int    `json:"reject"`
			Uprate       string `json:"uprate"`
			Signal       string `json:"signal"`
			ClientType   string `json:"client_type"`
			Bssid        string `json:"bssid"`
			AuthType     int    `json:"auth_type"`
			WebID        int    `json:"webid"`
			Comment      string `json:"comment"`
			Username     string `json:"username"`
			PPPType      string `json:"ppptype"`
			ApMac        string `json:"apmac"`
			Upload       int    `json:"upload"`
			Ssid         string `json:"ssid"`
			Frequencies  string `json:"frequencies"`
			Uptime       string `json:"uptime"`
			Id           int    `json:"id"`
			IpAddrInt    int64  `json:"ip_addr_int"`
			ConnectNum   int    `json:"connect_num"`
			IpAddr       string `json:"ip_addr"`
			Download     int    `json:"download"`
			TotalUp      int64  `json:"total_up"`
			TotalDown    int64  `json:"total_down"`
			ClientDevice string `json:"client_device"`
			Timestamp    int    `json:"timestamp"`
		} `json:"data"`
	} `json:"Data"`
}

type MonitorIFaceShowResp struct {
	CallResp
	Data struct {
		IFaceCheck []struct {
			Id              int    `json:"id"`
			Interface       string `json:"interface"`
			ParentInterface string `json:"parent_interface"`
			IpAddr          string `json:"ip_addr"`
			Gateway         string `json:"gateway"`
			Internet        string `json:"internet"`
			UpdateTime      string `json:"updatetime"`
			AutoSwitch      string `json:"auto_switch"`
			Result          string `json:"result"`
			ErrMsg          string `json:"errmsg"`
			Comment         string `json:"comment"`
		} `json:"iface_check"`
		IFaceStream []struct {
			Interface   string `json:"interface"`
			Comment     string `json:"comment"`
			IpAddr      string `json:"ip_addr"`
			ConnectNum  string `json:"connect_num"`
			Upload      int    `json:"upload"`
			Download    int    `json:"download"`
			TotalUp     int64  `json:"total_up"`
			TotalDown   int64  `json:"total_down"`
			UpDropped   int    `json:"updropped"`
			DownDropped int    `json:"downdropped"`
			UpPacked    int    `json:"uppacked"`
			DownPacked  int    `json:"downpacked"`
		} `json:"iface_stream"`
	} `json:"Data"`
}
