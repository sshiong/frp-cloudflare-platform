// Package dns 提供 DNS 记录管理和域名处理功能。
// 支持域名规范化、区域提取和 Cloudflare DNS 操作。
package dns

import (
	"fmt"
	"strings"

	"golang.org/x/net/idna"
)

// Record DNS 记录类型。
type Record struct {
	Type    string `json:"type"`    // A, AAAA, CNAME, TXT, etc.
	Name    string `json:"name"`    // 记录名称
	Content string `json:"content"` // 记录内容
	TTL     int    `json:"ttl"`     // TTL（秒），1 = 自动
	Proxied bool   `json:"proxied"` // 是否启用 Cloudflare 代理
}

// NormalizeDomain 规范化域名（IDNA/Punycode 转换）。
// 输入: Unicode 或 ASCII 域名
// 输出: 小写的 ASCII Punycode 域名
func NormalizeDomain(domain string) (string, error) {
	domain = strings.TrimSpace(domain)
	domain = strings.TrimSuffix(domain, ".")

	if domain == "" {
		return "", fmt.Errorf("empty domain")
	}

	// 使用 IDNA 转换为 ASCII（Punycode）
	ascii, err := idna.ToASCII(domain)
	if err != nil {
		return "", fmt.Errorf("idna conversion: %w", err)
	}

	return strings.ToLower(ascii), nil
}

// ExtractZone 从 FQDN 中提取可能的域名区域。
// 使用标签后缀匹配：逐级去掉左侧标签，返回第一个匹配的区域。
// 例如: "sub.example.co.uk" -> ["example.co.uk", "co.uk"]
func ExtractZone(fqdn string) []string {
	fqdn = strings.ToLower(strings.TrimSuffix(fqdn, "."))
	parts := strings.Split(fqdn, ".")

	var zones []string
	for i := 1; i < len(parts); i++ {
		zone := strings.Join(parts[i:], ".")
		zones = append(zones, zone)
	}
	return zones
}

// ExtractSubdomain 提取子域名部分。
// 例如: ("sub.example.com", "example.com") -> "sub"
//       ("example.com", "example.com") -> ""
func ExtractSubdomain(fqdn, zone string) string {
	fqdn = strings.ToLower(strings.TrimSuffix(fqdn, "."))
	zone = strings.ToLower(strings.TrimSuffix(zone, "."))

	if fqdn == zone {
		return ""
	}
	suffix := "." + zone
	if strings.HasSuffix(fqdn, suffix) {
		return strings.TrimSuffix(fqdn, suffix)
	}
	return fqdn
}

// ValidateDomainName 验证域名格式。
func ValidateDomainName(domain string) error {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return fmt.Errorf("domain is empty")
	}
	if len(domain) > 253 {
		return fmt.Errorf("domain too long: %d > 253", len(domain))
	}

	// 基本格式验证
	labels := strings.Split(domain, ".")
	for _, label := range labels {
		if label == "" {
			return fmt.Errorf("empty label in domain")
		}
		if len(label) > 63 {
			return fmt.Errorf("label too long: %s", label)
		}
	}

	if len(labels) < 2 {
		return fmt.Errorf("domain must have at least two labels")
	}

	return nil
}

// DNSProvider DNS 提供者接口。
type DNSProvider interface {
	// GetZoneID 获取域名对应的 Zone ID。
	GetZoneID(domain string) (string, error)
	// CreateRecord 创建 DNS 记录。
	CreateRecord(zoneID string, record Record) (string, error)
	// UpdateRecord 更新 DNS 记录。
	UpdateRecord(zoneID, recordID string, record Record) error
	// DeleteRecord 删除 DNS 记录。
	DeleteRecord(zoneID, recordID string) error
	// ListRecords 列出 DNS 记录。
	ListRecords(zoneID, name, recordType string) ([]DNSRecordResult, error)
}

// DNSRecordResult DNS 记录查询结果。
type DNSRecordResult struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied"`
}

// EnsureARecord 确保 A 记录存在。
// 如果不存在则创建，如果内容不同则更新。
func EnsureARecord(provider DNSProvider, zoneID, name, ip string, proxied bool) error {
	records, err := provider.ListRecords(zoneID, name, "A")
	if err != nil {
		return fmt.Errorf("list records: %w", err)
	}

	record := Record{
		Type:    "A",
		Name:    name,
		Content: ip,
		TTL:     1, // 自动
		Proxied: proxied,
	}

	if len(records) == 0 {
		// 创建新记录
		_, err := provider.CreateRecord(zoneID, record)
		return err
	}

	// 更新现有记录
	for _, r := range records {
		if r.Content != ip || r.Proxied != proxied {
			if err := provider.UpdateRecord(zoneID, r.ID, record); err != nil {
				return err
			}
		}
	}
	return nil
}

// EnsureCNAMERecord 确保 CNAME 记录存在。
func EnsureCNAMERecord(provider DNSProvider, zoneID, name, target string, proxied bool) error {
	records, err := provider.ListRecords(zoneID, name, "CNAME")
	if err != nil {
		return fmt.Errorf("list records: %w", err)
	}

	record := Record{
		Type:    "CNAME",
		Name:    name,
		Content: target,
		TTL:     1,
		Proxied: proxied,
	}

	if len(records) == 0 {
		_, err := provider.CreateRecord(zoneID, record)
		return err
	}

	for _, r := range records {
		if r.Content != target || r.Proxied != proxied {
			if err := provider.UpdateRecord(zoneID, r.ID, record); err != nil {
				return err
			}
		}
	}
	return nil
}

// RemoveRecord 移除指定名称和类型的 DNS 记录。
func RemoveRecord(provider DNSProvider, zoneID, name, recordType string) error {
	records, err := provider.ListRecords(zoneID, name, recordType)
	if err != nil {
		return err
	}
	for _, r := range records {
		if err := provider.DeleteRecord(zoneID, r.ID); err != nil {
			return err
		}
	}
	return nil
}
