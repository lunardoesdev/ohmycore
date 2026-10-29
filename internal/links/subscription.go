package links

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type SubscriptionCheckResult struct {
	Nodes  []SupportResult `json:"nodes"`
	Errors []string        `json:"errors,omitempty"`
}

func CheckXraySubscriptionSupport(data []byte) SubscriptionCheckResult {
	lines := strings.Split(string(data), "\n")
	var res SubscriptionCheckResult
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		r, err := CheckXrayLinkSupport(l)
		if err != nil {
			res.Errors = append(res.Errors, err.Error())
			continue
		}
		res.Nodes = append(res.Nodes, r)
	}
	return res
}

func CheckMihomoSubscriptionSupport(data []byte) SubscriptionCheckResult {
	lines := strings.Split(string(data), "\n")
	var res SubscriptionCheckResult
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		r, err := CheckMihomoLinkSupport(l)
		if err != nil {
			res.Errors = append(res.Errors, err.Error())
			continue
		}
		res.Nodes = append(res.Nodes, r)
	}
	return res
}

func FetchSubscription(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ohmycore")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, err
	}
	return b, nil
}

func SubscriptionToXrayConfig(data []byte) ([]byte, error) {
	lines := strings.Split(string(data), "\n")
	var nodes []Node
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		r, err := CheckXrayLinkSupport(l)
		if err != nil || !r.Supported {
			continue
		}
		u, _ := url.Parse(l)
		nodes = append(nodes, Node{Tag: r.Scheme, Protocol: r.Scheme, Server: u.Hostname(), Port: atoi(u.Port())})
	}
	return GenerateXrayConfig(nodes)
}

func SubscriptionToMihomoConfig(data []byte) ([]byte, error) {
	lines := strings.Split(string(data), "\n")
	var nodes []Node
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		r, err := CheckMihomoLinkSupport(l)
		if err != nil || !r.Supported {
			continue
		}
		u, _ := url.Parse(l)
		nodes = append(nodes, Node{Tag: r.Scheme, Protocol: r.Scheme, Server: u.Hostname(), Port: atoi(u.Port())})
	}
	return GenerateMihomoConfig(nodes)
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return n
		}
		n = n*10 + int(c-'0')
	}
	return n
}
