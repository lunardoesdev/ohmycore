package links

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type SupportResult struct {
	Supported bool   `json:"supported"`
	Scheme    string `json:"scheme"`
	Reason    string `json:"reason,omitempty"`
}

type Node struct {
	Tag      string `json:"tag"`
	Protocol string `json:"protocol"`
	Server   string `json:"server"`
	Port     int    `json:"port"`
	UUID     string `json:"uuid,omitempty"`
	Password string `json:"password,omitempty"`
}

func parseScheme(link string) (string, error) {
	u, err := url.Parse(link)
	if err != nil {
		return "", err
	}
	return u.Scheme, nil
}

func CheckXrayLinkSupport(link string) (SupportResult, error) {
	scheme, err := parseScheme(link)
	if err != nil {
		return SupportResult{}, err
	}
	switch scheme {
	case "vless", "vmess", "trojan", "ss", "socks", "http", "hysteria2", "hy2", "tuic":
		return SupportResult{Supported: true, Scheme: scheme}, nil
	default:
		return SupportResult{Supported: false, Scheme: scheme, Reason: "unsupported scheme for xray"}, nil
	}
}

func CheckMihomoLinkSupport(link string) (SupportResult, error) {
	scheme, err := parseScheme(link)
	if err != nil {
		return SupportResult{}, err
	}
	switch scheme {
	case "vless", "vmess", "trojan", "ss", "socks", "http", "hysteria2", "hy2", "tuic", "anytls", "wireguard":
		return SupportResult{Supported: true, Scheme: scheme}, nil
	default:
		return SupportResult{Supported: false, Scheme: scheme, Reason: "unsupported scheme for mihomo"}, nil
	}
}

func LinkToXrayConfig(link string) ([]byte, error) {
	r, err := CheckXrayLinkSupport(link)
	if err != nil {
		return nil, err
	}
	if !r.Supported {
		return nil, fmt.Errorf("unsupported: %s", r.Reason)
	}
	u, _ := url.Parse(link)
	cfg := map[string]any{
		"log": map[string]any{"loglevel": "warning"},
		"inbounds": []map[string]any{{"port": 10808, "protocol": "socks", "listen": "127.0.0.1"}},
		"outbounds": []map[string]any{{"protocol": r.Scheme, "settings": map[string]any{"address": u.Hostname(), "port": u.Port()}}},
	}
	return json.Marshal(cfg)
}

func LinkToMihomoConfig(link string) ([]byte, error) {
	r, err := CheckMihomoLinkSupport(link)
	if err != nil {
		return nil, err
	}
	if !r.Supported {
		return nil, fmt.Errorf("unsupported: %s", r.Reason)
	}
	u, _ := url.Parse(link)
	_ = u
	// minimal yaml
	out := fmt.Sprintf("mixed-port: 7890\nmode: rule\nproxies:\n  - name: imported\n    type: %s\n    server: %s\n    port: %s\n", r.Scheme, u.Hostname(), u.Port())
	return []byte(out), nil
}

func ParseSubscription(data []byte) ([][]string, error) {
	// placeholder
	lines := strings.Split(string(data), "\n")
	var out [][]string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		out = append(out, []string{l})
	}
	return out, nil
}

var _ = strings.TrimSpace
