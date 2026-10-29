package links

import (
	"encoding/json"
	"fmt"
	"strings"
)

// GenerateXrayConfig builds a runnable Xray JSON config from nodes.
func GenerateXrayConfig(nodes []Node) ([]byte, error) {
	outbounds := make([]map[string]any, 0, len(nodes))
	for _, n := range nodes {
		outbounds = append(outbounds, map[string]any{
			"tag":      n.Tag,
			"protocol": n.Protocol,
			"settings": map[string]any{
				"address": n.Server,
				"port":    n.Port,
			},
		})
	}
	if len(outbounds) == 0 {
		outbounds = append(outbounds, map[string]any{"protocol": "freedom", "tag": "direct"})
	}
	cfg := map[string]any{
		"log":      map[string]any{"loglevel": "warning"},
		"inbounds": []map[string]any{{"port": 10808, "protocol": "socks", "listen": "127.0.0.1"}},
		"outbounds": outbounds,
	}
	return json.Marshal(cfg)
}

// GenerateMihomoConfig builds a runnable Mihomo YAML config from nodes.
func GenerateMihomoConfig(nodes []Node) ([]byte, error) {
	var b strings.Builder
	b.WriteString("mixed-port: 7890\n")
	b.WriteString("mode: rule\n")
	b.WriteString("log-level: info\n")
	b.WriteString("proxies:\n")
	for _, n := range nodes {
		b.WriteString(fmt.Sprintf("  - name: %s\n    type: %s\n    server: %s\n    port: %d\n", n.Tag, n.Protocol, n.Server, n.Port))
	}
	if len(nodes) == 0 {
		b.WriteString("  - name: DIRECT\n    type: direct\n")
	}
	b.WriteString("proxy-groups:\n")
	b.WriteString("  - name: ohmycore-managed-group\n    type: select\n    proxies:\n")
	for _, n := range nodes {
		b.WriteString(fmt.Sprintf("      - %s\n", n.Tag))
	}
	if len(nodes) == 0 {
		b.WriteString("      - DIRECT\n")
	}
	b.WriteString("rules:\n")
	b.WriteString("  - MATCH,ohmycore-managed-group\n")
	return []byte(b.String()), nil
}

// GenerateConfig is a convenience dispatcher.
func GenerateConfig(kernel string, nodes []Node) ([]byte, error) {
	switch kernel {
	case "xray":
		return GenerateXrayConfig(nodes)
	case "mihomo":
		return GenerateMihomoConfig(nodes)
	default:
		return nil, fmt.Errorf("unknown kernel %s", kernel)
	}
}
