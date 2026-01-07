// Package komari 提供 Komari API 客户端和 IP CIDR 规则生成器。
package komari

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// DefaultBaseURL 是 Komari API 的默认基础 URL
	// 留空以强制要求配置
	DefaultBaseURL = ""
)

// Client 是 Komari API 客户端
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewClient 创建一个新的 Komari API 客户端
// baseURL 为空时使用默认地址
func NewClient(apiKey string, baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	// 规范化 BaseURL: 自动追加 /api 后缀
	baseURL = strings.TrimSuffix(baseURL, "/")
	if baseURL != "" && !strings.HasSuffix(baseURL, "/api") {
		baseURL += "/api"
	}

	return &Client{
		apiKey:  apiKey,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// doRequest 执行 HTTP 请求
func (c *Client) doRequest(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "*/*")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API 请求失败: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// GetClients 获取所有服务器列表
func (c *Client) GetClients() ([]KomariClient, error) {
	url := c.baseURL + "/admin/client/list"
	data, err := c.doRequest(url)
	if err != nil {
		return nil, fmt.Errorf("获取服务器列表失败: %w", err)
	}

	var clients []KomariClient
	if err := json.Unmarshal(data, &clients); err != nil {
		return nil, fmt.Errorf("解析服务器列表失败: %w", err)
	}

	return clients, nil
}

// GetPing 获取指定服务器的 ping 数据
func (c *Client) GetPing(uuid string) (*PingResponse, error) {
	url := fmt.Sprintf("%s/records/ping?uuid=%s&hours=1", c.baseURL, uuid)
	data, err := c.doRequest(url)
	if err != nil {
		return nil, fmt.Errorf("获取 ping 数据失败: %w", err)
	}

	var resp PingResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("解析 ping 数据失败: %w", err)
	}

	return &resp, nil
}

// GetAveragePing 获取服务器的最低平均 ping 值
// 按 task_id 分组计算每组平均值，返回最低的那个
// 如果无法获取或数据为空，返回 -1
func (c *Client) GetAveragePing(uuid string) int {
	resp, err := c.GetPing(uuid)
	if err != nil || resp.Status != "success" {
		return -1
	}

	if len(resp.Data.Records) == 0 {
		return -1
	}

	// 按 task_id 分组统计
	taskPings := make(map[int][]int)
	for _, record := range resp.Data.Records {
		taskPings[record.TaskID] = append(taskPings[record.TaskID], record.Value)
	}

	// 计算每个 task 的平均值，取最小值
	minAvg := -1
	for _, pings := range taskPings {
		if len(pings) == 0 {
			continue
		}
		sum := 0
		for _, p := range pings {
			sum += p
		}
		avg := sum / len(pings)
		if minAvg == -1 || avg < minAvg {
			minAvg = avg
		}
	}

	return minAvg
}
