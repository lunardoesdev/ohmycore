package mihomo

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/metacubex/mihomo/config"
	"github.com/metacubex/mihomo/hub/executor"
	"ohmycore/core"
)

func (a *Adapter) loadConfig(inst *core.Instance) (map[string]any, error) {
	inst.Mu.RLock()
	h, _ := inst.Handle.(*handle)
	inst.Mu.RUnlock()
	if h == nil {
		return nil, core.ErrNotRunning
	}
	var cfg map[string]any
	if err := yaml.Unmarshal(h.config, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (a *Adapter) saveConfig(inst *core.Instance, cfg map[string]any) error {
	inst.Mu.RLock()
	h, _ := inst.Handle.(*handle)
	inst.Mu.RUnlock()
	if h == nil {
		return core.ErrNotRunning
	}
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	// validate
	parsed, err := executor.ParseWithBytes(b)
	if err != nil {
		return err
	}
	// persist
	cfgPath := filepath.Join(inst.WorkDir, "config.yaml")
	tmp := cfgPath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, cfgPath); err != nil {
		return err
	}
	executor.ApplyConfig(parsed, false)
	inst.Mu.Lock()
	if hh, ok := inst.Handle.(*handle); ok {
		hh.config = b
	}
	inst.Mu.Unlock()
	return nil
}

func (a *Adapter) AddProxy(inst *core.Instance, proxy map[string]any) error {
	cfg, err := a.loadConfig(inst)
	if err != nil {
		return err
	}
	proxies, _ := cfg["proxies"].([]any)
	name, _ := proxy["name"].(string)
	if name == "" {
		return fmt.Errorf("proxy name required")
	}
	for _, p := range proxies {
		if m, ok := p.(map[string]any); ok {
			if m["name"] == name {
				return fmt.Errorf("proxy %s already exists", name)
			}
		}
	}
	cfg["proxies"] = append(proxies, proxy)
	return a.saveConfig(inst, cfg)
}

func (a *Adapter) RemoveProxy(inst *core.Instance, name string) error {
	cfg, err := a.loadConfig(inst)
	if err != nil {
		return err
	}
	proxies, _ := cfg["proxies"].([]any)
	var filtered []any
	found := false
	for _, p := range proxies {
		if m, ok := p.(map[string]any); ok && m["name"] == name {
			found = true
			continue
		}
		filtered = append(filtered, p)
	}
	if !found {
		return fmt.Errorf("proxy %s not found", name)
	}
	cfg["proxies"] = filtered
	return a.saveConfig(inst, cfg)
}

func (a *Adapter) UpdateProxy(inst *core.Instance, name string, proxy map[string]any) error {
	if err := a.RemoveProxy(inst, name); err != nil {
		return err
	}
	return a.AddProxy(inst, proxy)
}

var _ = config.Init
