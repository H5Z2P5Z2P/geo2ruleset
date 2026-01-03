# Surge-Geosite Go 版本

Geosite Ruleset Converter for Surge - Go 实现

本项目重写自 https://github.com/xxxbrian/Surge-Geosite , 感谢原作者的开源贡献。

## 功能

将 [v2fly/domain-list-community](https://github.com/v2fly/domain-list-community) 的 geosite 数据动态转换为 Surge Ruleset 格式。

## 安装

```bash
go build -o surge-geosite
```

## 使用

```bash
# 默认端口 8080
./surge-geosite

# 自定义端口
./surge-geosite -port 3000

# 自定义缓存 TTL
./surge-geosite -zip-ttl 1h -result-ttl 24h

# 启用 ZIP 磁盘缓存与定时刷新
./surge-geosite -zip-cache-path ./data/zip-cache.gob -zip-refresh-interval 30m
```

## API 端点

| 端点 | 描述 |
|------|------|
| `GET /` | 重定向到 GitHub 仓库 |
| `GET /geosite` | 返回所有可用规则的 JSON 索引 |
| `GET /geosite/:name` | 获取指定规则列表 |
| `GET /geosite/:name@filter` | 获取带过滤器的规则列表 |
| `GET /geosite/surge/:name` | 获取 Surge 规则列表（别名） |
| `GET /geosite/surge/:name@filter` | 获取 Surge 规则列表（别名，带过滤器） |
| `GET /geosite/mihomo/:name` | 获取 Mihomo 规则列表（classical） |
| `GET /geosite/mihomo/:name@filter` | 获取 Mihomo 规则列表（带过滤器） |
| `GET /geosite/egern/:name` | 获取 Egern 规则集合（YAML） |
| `GET /geosite/egern/:name@filter` | 获取 Egern 规则集合（带过滤器） |
| `GET /misc/:category/:name` | 获取自定义规则列表 |

## 示例

```bash
# 获取 Google 规则
curl http://localhost:8080/geosite/google

# 获取 Apple 中国区规则
curl http://localhost:8080/geosite/apple@cn

# 获取微信规则
curl http://localhost:8080/misc/wechat/wechat
```

## Docker Compose

```bash
docker compose up -d --build
```

## 规则转换

| v2fly 格式 | Surge 格式 |
|-----------|-----------|
| `domain:example.com` | `DOMAIN-SUFFIX,example.com` |
| `full:example.com` | `DOMAIN,example.com` |
| `keyword:example` | `DOMAIN-KEYWORD,example` |
| `regexp:.*` | `DOMAIN-WILDCARD,*` |
| `example.com` | `DOMAIN-SUFFIX,example.com` |
