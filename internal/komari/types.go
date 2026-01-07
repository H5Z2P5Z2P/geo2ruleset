// Package komari 提供 Komari API 客户端和 IP CIDR 规则生成器。
package komari

// KomariClient 表示 Komari 服务器信息
type KomariClient struct {
	UUID   string `json:"uuid"`
	Name   string `json:"name"`
	IPv4   string `json:"ipv4,omitempty"`
	IPv6   string `json:"ipv6,omitempty"`
	Region string `json:"region"` // emoji 地区标识，如 🇯🇵
	Group  string `json:"group"`  // 地区分组，如 JP, HK, US
}

// ClientListResponse 表示服务器列表 API 响应
type ClientListResponse []KomariClient

// PingRecord 表示单条 ping 记录
type PingRecord struct {
	TaskID int    `json:"task_id"`
	Time   string `json:"time"`
	Value  int    `json:"value"`  // 延迟，单位 ms
	Client string `json:"client"` // 服务器 UUID
}

// PingBasicInfo 表示 ping 基础信息
type PingBasicInfo struct {
	Client string `json:"client"`
	Loss   int    `json:"loss"`
	Min    int    `json:"min"`
	Max    int    `json:"max"`
}

// PingData 表示 ping 数据
type PingData struct {
	Count     int             `json:"count"`
	BasicInfo []PingBasicInfo `json:"basic_info"`
	Records   []PingRecord    `json:"records"`
}

// PingResponse 表示 ping API 响应
type PingResponse struct {
	Status  string   `json:"status"`
	Message string   `json:"message"`
	Data    PingData `json:"data"`
}

// IPCIDR 表示一条 IP CIDR 规则
type IPCIDR struct {
	IP      string // IP 地址
	IsIPv6  bool   // 是否为 IPv6
	Comment string // 注释（服务器名称）
}
