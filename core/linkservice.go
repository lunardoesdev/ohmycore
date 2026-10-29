package core

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"ohmycore/internal/links"
)

// LinkResolution is the result of resolving a link to a backend.
type LinkResolution struct {
	Kernel  Kernel
	Node    links.Node
	Support links.SupportResult
}

// LinkService provides data-oriented operations on links.
type LinkService struct {
	Manager *Manager
}

// NewLinkService creates a LinkService.
func NewLinkService(m *Manager) *LinkService {
	return &LinkService{Manager: m}
}

// ProxyManager is an optional adapter capability.
type ProxyManager interface {
	AddProxy(inst *Instance, proxy map[string]any) error
	RemoveProxy(inst *Instance, name string) error
	SelectProxy(inst *Instance, group, name string) error
	Delay(inst *Instance, name, testURL string, timeout time.Duration) (int, error)
}

// Prober is an optional adapter capability.
type Prober interface {
	Ping(ctx context.Context, cfg []byte, outboundTag, testURL string, timeout time.Duration) (int64, error)
}

// Resolve determines which backend supports the link (Xray preferred).
func (s *LinkService) Resolve(link string) (LinkResolution, error) {
	rx, err := links.CheckXrayLinkSupport(link)
	if err != nil {
		return LinkResolution{}, err
	}
	if rx.Supported {
		u, _ := url.Parse(link)
		return LinkResolution{Kernel: KernelXray, Node: links.Node{Tag: rx.Scheme, Protocol: rx.Scheme, Server: u.Hostname(), Port: atoi(u.Port())}, Support: rx}, nil
	}
	rm, err := links.CheckMihomoLinkSupport(link)
	if err != nil {
		return LinkResolution{}, err
	}
	if rm.Supported {
		u, _ := url.Parse(link)
		return LinkResolution{Kernel: KernelMihomo, Node: links.Node{Tag: rm.Scheme, Protocol: rm.Scheme, Server: u.Hostname(), Port: atoi(u.Port())}, Support: rm}, nil
	}
	return LinkResolution{}, ErrUnsupportedLink
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

// Import imports a link into the appropriate backend.
func (s *LinkService) Import(link string) (string, error) {
	res, err := s.Resolve(link)
	if err != nil {
		return "", err
	}
	if res.Kernel == KernelXray {
		cfg, err := links.LinkToXrayConfig(link)
		if err != nil {
			return "", err
		}
		inst, err := s.Manager.Create(KernelXray, cfg)
		if err != nil {
			return "", err
		}
		if err := s.Manager.Start(inst.ID); err != nil {
			return "", err
		}
		return inst.ID, nil
	}
	var inst *Instance
	for _, i := range s.Manager.List() {
		if i.Kernel == KernelMihomo && i.Status == StatusRunning {
			inst = i
			break
		}
	}
	if inst == nil {
		cfg, err := links.LinkToMihomoConfig(link)
		if err != nil {
			return "", err
		}
		ni, err := s.Manager.Create(KernelMihomo, cfg)
		if err != nil {
			return "", err
		}
		if err := s.Manager.Start(ni.ID); err != nil {
			return "", err
		}
		inst = ni
	}
	a, err := s.Manager.Adapter(KernelMihomo)
	if err != nil {
		return "", err
	}
	pm, ok := a.(ProxyManager)
	if !ok {
		return "", fmt.Errorf("adapter does not support proxy management")
	}
	nodeMap := map[string]any{"name": res.Node.Tag, "type": res.Node.Protocol, "server": res.Node.Server, "port": res.Node.Port}
	if err := pm.AddProxy(inst, nodeMap); err != nil {
		return "", err
	}
	return inst.ID, nil
}

// Test tests a link without changing active selection.
func (s *LinkService) Test(link string, testURL string) (int, error) {
	res, err := s.Resolve(link)
	if err != nil {
		return 0, err
	}
	if res.Kernel == KernelXray {
		cfg, err := links.LinkToXrayConfig(link)
		if err != nil {
			return 0, err
		}
		a, err := s.Manager.Adapter(KernelXray)
		if err != nil {
			return 0, err
		}
		p, ok := a.(Prober)
		if !ok {
			return 0, fmt.Errorf("adapter does not support probing")
		}
		ms, err := p.Ping(context.Background(), cfg, res.Node.Tag, testURL, 0)
		if err != nil {
			return 0, err
		}
		return int(ms), nil
	}
	var inst *Instance
	for _, i := range s.Manager.List() {
		if i.Kernel == KernelMihomo && i.Status == StatusRunning {
			inst = i
			break
		}
	}
	if inst == nil {
		return 0, fmt.Errorf("no running mihomo instance")
	}
	a, err := s.Manager.Adapter(KernelMihomo)
	if err != nil {
		return 0, err
	}
	pm, ok := a.(ProxyManager)
	if !ok {
		return 0, fmt.Errorf("adapter does not support proxy management")
	}
	return pm.Delay(inst, res.Node.Tag, testURL, 5*time.Second)
}

// Activate makes the link the current proxy.
func (s *LinkService) Activate(link string) (string, error) {
	res, err := s.Resolve(link)
	if err != nil {
		return "", err
	}
	id, err := s.Import(link)
	if err != nil {
		return "", err
	}
	inst, _ := s.Manager.Get(id)
	if inst != nil && inst.Kernel == KernelMihomo {
		a, _ := s.Manager.Adapter(KernelMihomo)
		if pm, ok := a.(ProxyManager); ok {
			_ = pm.SelectProxy(inst, "ohmycore-managed-group", res.Node.Tag)
		}
	}
	return id, nil
}
