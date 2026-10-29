//go:build cgo

package export

/*
#include <stdlib.h>
*/
import "C"

import (
	"encoding/json"
	"context"
	"os"
	"path/filepath"
	"time"
	"sync"
	"unsafe"

	"ohmycore/core"
	"ohmycore/internal/links"
	"ohmycore/internal/mihomo"
	"ohmycore/internal/xray"
)

type resp struct {
	Success bool   `json:"success"`
	Data    any    `json:"data"`
	Error   string `json:"error"`
}

var (
	mgrOnce sync.Once
	mgr     *core.Manager
)

func getManager() *core.Manager {
	mgrOnce.Do(func() {
		base := os.Getenv("OHMYCORE_BASE_DIR")
		if base == "" {
			h, _ := os.UserHomeDir()
			base = filepath.Join(h, ".ohmycore")
		}
		m := core.NewManager(base)
		m.RegisterAdapter(core.KernelXray, xray.New())
		m.RegisterAdapter(core.KernelMihomo, mihomo.New())
		mgr = m
	})
	return mgr
}

//export CGoInvoke
func CGoInvoke(requestJSON *C.char) *C.char {
	req := C.GoString(requestJSON)
	var r struct {
		APIVersion int             `json:"apiVersion"`
		Method     string          `json:"method"`
		Payload    json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal([]byte(req), &r); err != nil {
		b, _ := json.Marshal(resp{Success: false, Error: err.Error()})
		return C.CString(string(b))
	}
	if r.APIVersion != 3 {
		b, _ := json.Marshal(resp{Success: false, Error: "unsupported apiVersion"})
		return C.CString(string(b))
	}
	m := getManager()
	switch r.Method {
	case "createInstance":
		var p struct {
			Kernel string `json:"kernel"`
			Config string `json:"config"`
		}
		json.Unmarshal(r.Payload, &p)
		inst, err := m.Create(core.Kernel(p.Kernel), []byte(p.Config))
		if err != nil {
			b, _ := json.Marshal(resp{Success: false, Error: err.Error()})
			return C.CString(string(b))
		}
		b, _ := json.Marshal(resp{Success: true, Data: inst})
		return C.CString(string(b))
	case "startInstance":
		var p struct {
			ID string `json:"id"`
		}
		json.Unmarshal(r.Payload, &p)
		if err := m.Start(p.ID); err != nil {
			b, _ := json.Marshal(resp{Success: false, Error: err.Error()})
			return C.CString(string(b))
		}
		b, _ := json.Marshal(resp{Success: true, Data: struct{}{}})
		return C.CString(string(b))
	case "stopInstance":
		var p struct {
			ID string `json:"id"`
		}
		json.Unmarshal(r.Payload, &p)
		if err := m.Stop(p.ID); err != nil {
			b, _ := json.Marshal(resp{Success: false, Error: err.Error()})
			return C.CString(string(b))
		}
		b, _ := json.Marshal(resp{Success: true, Data: struct{}{}})
		return C.CString(string(b))
	case "removeInstance":
		var p struct {
			ID string `json:"id"`
		}
		json.Unmarshal(r.Payload, &p)
		if err := m.Remove(p.ID); err != nil {
			b, _ := json.Marshal(resp{Success: false, Error: err.Error()})
			return C.CString(string(b))
		}
		b, _ := json.Marshal(resp{Success: true, Data: struct{}{}})
		return C.CString(string(b))
	case "listInstances":
		insts := m.List()
		b, _ := json.Marshal(resp{Success: true, Data: insts})
		return C.CString(string(b))
	case "getInstance":
		var p struct {
			ID string `json:"id"`
		}
		json.Unmarshal(r.Payload, &p)
		inst, err := m.Get(p.ID)
		if err != nil {
			b, _ := json.Marshal(resp{Success: false, Error: err.Error()})
			return C.CString(string(b))
		}
		b, _ := json.Marshal(resp{Success: true, Data: inst})
		return C.CString(string(b))
	case "reloadConfig":
		var p struct {
			ID     string `json:"id"`
			Config string `json:"config"`
		}
		json.Unmarshal(r.Payload, &p)
		if err := m.Reload(p.ID, []byte(p.Config)); err != nil {
			b, _ := json.Marshal(resp{Success: false, Error: err.Error()})
			return C.CString(string(b))
		}
		b, _ := json.Marshal(resp{Success: true, Data: struct{}{}})
		return C.CString(string(b))
	case "xrayPing":
		var p struct {
			Config      string `json:"config"`
			OutboundTag string `json:"outboundTag"`
			URL         string `json:"url"`
			Timeout     int    `json:"timeout"`
		}
		json.Unmarshal(r.Payload, &p)
		a, _ := m.Adapter(core.KernelXray)
		xa := a.(*xray.Adapter)
		ms, err := xa.Ping(context.Background(), []byte(p.Config), p.OutboundTag, p.URL, time.Duration(p.Timeout)*time.Millisecond)
		if err != nil {
			b, _ := json.Marshal(resp{Success: false, Error: err.Error()})
			return C.CString(string(b))
		}
		b, _ := json.Marshal(resp{Success: true, Data: map[string]int64{"ms": ms}})
		return C.CString(string(b))
	case "xraySpeedTest":
		var p struct {
			Config       string `json:"config"`
			OutboundTag  string `json:"outboundTag"`
			DownloadURL  string `json:"downloadUrl"`
			Duration     int    `json:"duration"`
		}
		json.Unmarshal(r.Payload, &p)
		a, _ := m.Adapter(core.KernelXray)
		xa := a.(*xray.Adapter)
		bps, err := xa.SpeedTest(context.Background(), []byte(p.Config), p.OutboundTag, p.DownloadURL, time.Duration(p.Duration)*time.Millisecond)
		if err != nil {
			b, _ := json.Marshal(resp{Success: false, Error: err.Error()})
			return C.CString(string(b))
		}
		b, _ := json.Marshal(resp{Success: true, Data: map[string]float64{"bytesPerSec": bps}})
		return C.CString(string(b))
	case "mihomoProxies":
		var p struct {
			ID string `json:"id"`
		}
		json.Unmarshal(r.Payload, &p)
		inst, err := m.Get(p.ID)
		if err != nil {
			b, _ := json.Marshal(resp{Success: false, Error: err.Error()})
			return C.CString(string(b))
		}
		a, _ := m.Adapter(core.KernelMihomo)
		ma := a.(*mihomo.Adapter)
		proxies, err := ma.Proxies(inst)
		if err != nil {
			b, _ := json.Marshal(resp{Success: false, Error: err.Error()})
			return C.CString(string(b))
		}
		b, _ := json.Marshal(resp{Success: true, Data: proxies})
		return C.CString(string(b))
	case "mihomoDelay":
		var p struct {
			ID      string `json:"id"`
			Proxy   string `json:"proxy"`
			URL     string `json:"url"`
			Timeout int    `json:"timeout"`
		}
		json.Unmarshal(r.Payload, &p)
		inst, err := m.Get(p.ID)
		if err != nil {
			b, _ := json.Marshal(resp{Success: false, Error: err.Error()})
			return C.CString(string(b))
		}
		a, _ := m.Adapter(core.KernelMihomo)
		ma := a.(*mihomo.Adapter)
		d, err := ma.Delay(inst, p.Proxy, p.URL, time.Duration(p.Timeout)*time.Millisecond)
		if err != nil {
			b, _ := json.Marshal(resp{Success: false, Error: err.Error()})
			return C.CString(string(b))
		}
		b, _ := json.Marshal(resp{Success: true, Data: map[string]int{"delay": d}})
		return C.CString(string(b))
	case "mihomoSpeedTest":
		var p struct {
			ID          string `json:"id"`
			Proxy       string `json:"proxy"`
			DownloadURL string `json:"downloadUrl"`
			MaxBytes    int64  `json:"maxBytes"`
			Duration    int    `json:"duration"`
		}
		json.Unmarshal(r.Payload, &p)
		inst, err := m.Get(p.ID)
		if err != nil {
			b, _ := json.Marshal(resp{Success: false, Error: err.Error()})
			return C.CString(string(b))
		}
		a, _ := m.Adapter(core.KernelMihomo)
		ma := a.(*mihomo.Adapter)
		bps, err := ma.SpeedTest(context.Background(), inst, p.Proxy, p.DownloadURL, p.MaxBytes, time.Duration(p.Duration)*time.Millisecond)
		if err != nil {
			b, _ := json.Marshal(resp{Success: false, Error: err.Error()})
			return C.CString(string(b))
		}
		b, _ := json.Marshal(resp{Success: true, Data: map[string]float64{"bytesPerSec": bps}})
		return C.CString(string(b))
	case "mihomoSelect":
		var p struct {
			ID    string `json:"id"`
			Group string `json:"group"`
			Proxy string `json:"proxy"`
		}
		json.Unmarshal(r.Payload, &p)
		inst, err := m.Get(p.ID)
		if err != nil {
			b, _ := json.Marshal(resp{Success: false, Error: err.Error()})
			return C.CString(string(b))
		}
		a, _ := m.Adapter(core.KernelMihomo)
		ma := a.(*mihomo.Adapter)
		if err := ma.SelectProxy(inst, p.Group, p.Proxy); err != nil {
			b, _ := json.Marshal(resp{Success: false, Error: err.Error()})
			return C.CString(string(b))
		}
		b, _ := json.Marshal(resp{Success: true, Data: struct{}{}})
		return C.CString(string(b))
	case "mihomoPatchConfig":
		var p struct {
			ID    string         `json:"id"`
			Patch map[string]any `json:"patch"`
		}
		json.Unmarshal(r.Payload, &p)
		inst, err := m.Get(p.ID)
		if err != nil {
			b, _ := json.Marshal(resp{Success: false, Error: err.Error()})
			return C.CString(string(b))
		}
		a, _ := m.Adapter(core.KernelMihomo)
		ma := a.(*mihomo.Adapter)
		if err := ma.PatchConfig(inst, p.Patch); err != nil {
			b, _ := json.Marshal(resp{Success: false, Error: err.Error()})
			return C.CString(string(b))
		}
		b, _ := json.Marshal(resp{Success: true, Data: struct{}{}})
		return C.CString(string(b))
	case "mihomoReloadProviders":
		var p struct {
			ID string `json:"id"`
		}
		json.Unmarshal(r.Payload, &p)
		inst, err := m.Get(p.ID)
		if err != nil {
			b, _ := json.Marshal(resp{Success: false, Error: err.Error()})
			return C.CString(string(b))
		}
		a, _ := m.Adapter(core.KernelMihomo)
		ma := a.(*mihomo.Adapter)
		if err := ma.ReloadProviders(inst); err != nil {
			b, _ := json.Marshal(resp{Success: false, Error: err.Error()})
			return C.CString(string(b))
		}
		b, _ := json.Marshal(resp{Success: true, Data: struct{}{}})
		return C.CString(string(b))
	case "checkXrayLinkSupport":
		var p struct {
			Link string `json:"link"`
		}
		json.Unmarshal(r.Payload, &p)
		res, err := links.CheckXrayLinkSupport(p.Link)
		if err != nil {
			b, _ := json.Marshal(resp{Success: false, Error: err.Error()})
			return C.CString(string(b))
		}
		b, _ := json.Marshal(resp{Success: true, Data: res})
		return C.CString(string(b))
	case "checkMihomoLinkSupport":
		var p struct {
			Link string `json:"link"`
		}
		json.Unmarshal(r.Payload, &p)
		res, err := links.CheckMihomoLinkSupport(p.Link)
		if err != nil {
			b, _ := json.Marshal(resp{Success: false, Error: err.Error()})
			return C.CString(string(b))
		}
		b, _ := json.Marshal(resp{Success: true, Data: res})
		return C.CString(string(b))
	case "linkToXrayConfig":
		var p struct {
			Link string `json:"link"`
		}
		json.Unmarshal(r.Payload, &p)
		cfg, err := links.LinkToXrayConfig(p.Link)
		if err != nil {
			b, _ := json.Marshal(resp{Success: false, Error: err.Error()})
			return C.CString(string(b))
		}
		b, _ := json.Marshal(resp{Success: true, Data: string(cfg)})
		return C.CString(string(b))
	case "linkToMihomoConfig":
		var p struct {
			Link string `json:"link"`
		}
		json.Unmarshal(r.Payload, &p)
		cfg, err := links.LinkToMihomoConfig(p.Link)
		if err != nil {
			b, _ := json.Marshal(resp{Success: false, Error: err.Error()})
			return C.CString(string(b))
		}
		b, _ := json.Marshal(resp{Success: true, Data: string(cfg)})
		return C.CString(string(b))
	case "version":
		b, _ := json.Marshal(resp{Success: true, Data: map[string]string{"xray": xray.New().Version(), "mihomo": "mihomo"}})
		return C.CString(string(b))
	default:
		b, _ := json.Marshal(resp{Success: false, Error: "unknown method"})
		return C.CString(string(b))
	}
}

//export CGoFree
func CGoFree(p *C.char) {
	C.free(unsafe.Pointer(p))
}

func Invoke(requestJSON string) string {
	p := CGoInvoke(C.CString(requestJSON))
	s := C.GoString(p)
	CGoFree(p)
	return s
}

