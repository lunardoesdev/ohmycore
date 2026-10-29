package mihomo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/metacubex/mihomo/config"
	C "github.com/metacubex/mihomo/constant"
	"github.com/metacubex/mihomo/hub"
	"github.com/metacubex/mihomo/hub/executor"
	"github.com/metacubex/mihomo/listener"

	"ohmycore/core"
)

type handle struct {
	controllerURL string
	secret        string
	port          int
}

var (
	mu      sync.Mutex
	current *core.Instance
)

type adapter struct{}

func New() core.Adapter { return &adapter{} }

func freePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

func (a *adapter) Start(inst *core.Instance, cfg []byte) error {
	mu.Lock()
	defer mu.Unlock()
	if current != nil {
		return core.ErrMihomoBusy
	}
	dir := inst.WorkDir
	C.SetHomeDir(dir)
	if err := config.Init(dir); err != nil {
		return err
	}
	port, err := freePort()
	if err != nil {
		return err
	}
	secret := "ohmycore"
	if err := hub.Parse(cfg, hub.WithExternalController(fmt.Sprintf("127.0.0.1:%d", port)), hub.WithSecret(secret)); err != nil {
		return err
	}
	inst.Mu.Lock()
	inst.Handle = &handle{controllerURL: fmt.Sprintf("http://127.0.0.1:%d", port), secret: secret, port: port}
	inst.Mu.Unlock()
	current = inst
	return nil
}

func (a *adapter) Stop(inst *core.Instance) error {
	mu.Lock()
	defer mu.Unlock()
	if current != inst {
		return core.ErrNotRunning
	}
	executor.Shutdown()
	current = nil
	inst.Mu.Lock()
	inst.Handle = nil
	inst.Mu.Unlock()
	return nil
}

func (a *adapter) Reload(inst *core.Instance, cfg []byte) error {
	if err := hub.Parse(cfg); err != nil {
		return err
	}
	return nil
}

func (a *adapter) Version() string { return "mihomo" }

func (a *adapter) do(inst *core.Instance, method, path string, body any) (*http.Response, error) {
	inst.Mu.RLock()
	h, _ := inst.Handle.(*handle)
	inst.Mu.RUnlock()
	if h == nil {
		return nil, core.ErrNotRunning
	}
	var r *http.Request
	var err error
	if body != nil {
		b, _ := json.Marshal(body)
		r, err = http.NewRequest(method, h.controllerURL+path, bytes.NewReader(b))
		if err == nil {
			r.Header.Set("Content-Type", "application/json")
		}
	} else {
		r, err = http.NewRequest(method, h.controllerURL+path, nil)
	}
	if err != nil {
		return nil, err
	}
	r.Header.Set("Authorization", "Bearer "+h.secret)
	return http.DefaultClient.Do(r)
}

func (a *adapter) Proxies(inst *core.Instance) (map[string]any, error) {
	resp, err := a.do(inst, "GET", "/proxies", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return out, nil
}

func (a *adapter) Delay(inst *core.Instance, name, testURL string, timeout time.Duration) (int, error) {
	resp, err := a.do(inst, "GET", fmt.Sprintf("/proxies/%s/delay?url=%s&timeout=%d", name, testURL, timeout.Milliseconds()), nil)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var out struct {
		Delay int `json:"delay"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	return out.Delay, nil
}

func (a *adapter) SelectProxy(inst *core.Instance, group, name string) error {
	resp, err := a.do(inst, "PUT", "/proxies/"+group, map[string]string{"name": name})
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("select failed: %d", resp.StatusCode)
	}
	return nil
}

func (a *adapter) PatchConfig(inst *core.Instance, patch map[string]any) error {
	resp, err := a.do(inst, "PATCH", "/configs", patch)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("patch failed: %d", resp.StatusCode)
	}
	return nil
}

func (a *adapter) ReloadProviders(inst *core.Instance) error {
	resp, err := a.do(inst, "GET", "/providers/proxies", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var out struct {
		Providers map[string]any `json:"providers"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	for name := range out.Providers {
		r, err := a.do(inst, "PUT", "/providers/proxies/"+name, nil)
		if err == nil {
			r.Body.Close()
		}
	}
	return nil
}

func (a *adapter) SpeedTest(ctx context.Context, inst *core.Instance, proxyName string, downloadURL string, maxBytes int64, duration time.Duration) (float64, error) {
	// placeholder: implement pinned listener later
	return 0, fmt.Errorf("not implemented")
}

var _ = listener.PatchInboundListeners
var _ = io.Discard
var _ = time.Second
var _ = http.StatusOK
