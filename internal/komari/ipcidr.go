// Package komari 提供 Komari API 客户端和 IP CIDR 规则生成器。
package komari

import (
	"fmt"
	"strings"
)

// 延迟阈值常量（毫秒）
const (
	ThresholdHK = 60  // 🇭🇰 香港
	ThresholdJP = 100 // 🇯🇵 日本
	ThresholdUS = 160 // 🇺🇸 美国
)

// FilterType 过滤类型
type FilterType string

const (
	FilterNone   FilterType = ""       // 无过滤，返回所有
	FilterDirect FilterType = "DIRECT" // 低延迟，满足阈值
	FilterProxy  FilterType = "PROXY"  // 高延迟，不满足阈值
)

// getThreshold 根据地区 emoji 获取延迟阈值
func getThreshold(region string) int {
	switch region {
	case "🇭🇰":
		return ThresholdHK
	case "🇯🇵":
		return ThresholdJP
	case "🇺🇸":
		return ThresholdUS
	default:
		// 其他地区默认归属 PROXY，设置阈值为 0 表示任何延迟都不满足
		return 0
	}
}

// isChinaRegion 判断是否为中国大陆地区
func isChinaRegion(region string) bool {
	return region == "🇨🇳"
}

// GenerateIPCIDR 生成 IP CIDR 规则列表
// filter: 过滤类型（空/DIRECT/PROXY）
// getPing: 获取服务器平均 ping 的函数，返回 -1 表示无法获取
func GenerateIPCIDR(clients []KomariClient, filter FilterType, getPing func(uuid string) int) []IPCIDR {
	var result []IPCIDR

	for _, client := range clients {
		// 排除中国大陆服务器
		if isChinaRegion(client.Region) {
			continue
		}

		// 跳过没有 IP 的服务器
		if client.IPv4 == "" && client.IPv6 == "" {
			continue
		}

		// 根据过滤类型判断是否需要检查延迟
		if filter != FilterNone && getPing != nil {
			threshold := getThreshold(client.Region)
			avgPing := getPing(client.UUID)

			// 判断是否满足阈值
			// threshold == 0 表示其他地区，统一归入 PROXY
			// avgPing == -1 表示无法获取 ping，也归入 PROXY
			meetThreshold := threshold > 0 && avgPing > 0 && avgPing <= threshold

			if filter == FilterDirect && !meetThreshold {
				continue
			}
			if filter == FilterProxy && meetThreshold {
				continue
			}
		}

		// 添加 IPv4 规则
		if client.IPv4 != "" {
			result = append(result, IPCIDR{
				IP:      client.IPv4 + "/32",
				IsIPv6:  false,
				Comment: client.Name,
			})
		}

		// 添加 IPv6 规则
		if client.IPv6 != "" {
			result = append(result, IPCIDR{
				IP:      normalizeIPv6(client.IPv6) + "/64",
				IsIPv6:  true,
				Comment: client.Name,
			})
		}
	}

	return result
}

// normalizeIPv6 规范化 IPv6 地址
// 提取 /64 前缀部分
func normalizeIPv6(ipv6 string) string {
	// 移除可能存在的 CIDR 后缀
	if idx := strings.Index(ipv6, "/"); idx != -1 {
		ipv6 = ipv6[:idx]
	}

	// 分割成 8 组
	parts := strings.Split(ipv6, ":")

	// 处理简写形式 ::
	if strings.Contains(ipv6, "::") {
		var expanded []string
		for i, part := range parts {
			if part == "" && i > 0 && i < len(parts)-1 {
				// 计算需要补齐的组数
				missing := 8 - len(parts) + 1
				for j := 0; j < missing; j++ {
					expanded = append(expanded, "0")
				}
			} else if part != "" {
				expanded = append(expanded, part)
			} else if i == 0 || i == len(parts)-1 {
				expanded = append(expanded, "0")
			}
		}
		parts = expanded
	}

	// 取前 4 组作为 /64 前缀
	if len(parts) >= 4 {
		return strings.Join(parts[:4], ":") + "::"
	}
	return ipv6
}

// RenderSurge 渲染 Surge 格式规则
func RenderSurge(cidrs []IPCIDR) string {
	var builder strings.Builder
	for _, cidr := range cidrs {
		if cidr.IsIPv6 {
			builder.WriteString(fmt.Sprintf("IP-CIDR6,%s\n", cidr.IP))
		} else {
			builder.WriteString(fmt.Sprintf("IP-CIDR,%s\n", cidr.IP))
		}
	}
	return strings.TrimRight(builder.String(), "\n")
}

// RenderMihomo 渲染 Mihomo 格式规则
// Mihomo 目前兼容 Surge 格式
func RenderMihomo(cidrs []IPCIDR) string {
	return RenderSurge(cidrs)
}

// RenderEgern 渲染为 Egern YAML 格式
func RenderEgern(cidrs []IPCIDR) string {
	var ipv4List, ipv6List []string

	for _, cidr := range cidrs {
		if cidr.IsIPv6 {
			ipv6List = append(ipv6List, cidr.IP)
		} else {
			ipv4List = append(ipv4List, cidr.IP)
		}
	}

	var b strings.Builder

	if len(ipv4List) > 0 {
		b.WriteString("ip_cidr_set:\n")
		for _, ip := range ipv4List {
			b.WriteString("  - ")
			b.WriteString(ip)
			b.WriteString("\n")
		}
	}

	if len(ipv6List) > 0 {
		b.WriteString("ip_cidr6_set:\n")
		for _, ip := range ipv6List {
			b.WriteString("  - ")
			b.WriteString(ip)
			b.WriteString("\n")
		}
	}

	return strings.TrimRight(b.String(), "\n")
}
