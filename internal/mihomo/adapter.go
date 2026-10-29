package mihomo
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/metacubex/mihomo/config"
	C "github.com/metacubex/mihomo/constant"
	"github.com/metacubex/mihomo/hub"
	"github.com/metacubex/mihomo/hub/executor"
	"github.com/metacubex/mihomo/listener"
	"github.com/metacubex/mihomo/tunnel"

	"ohmycore/core"
)

type handle struct {
	controllerURL string
	port          int
	secret        string
	config        []byte
}

var (
	mu      sync.Mutex
	current *core.Instance
)
var speedTestMu sync.Mutex
// removed duplicate

type Adapter struct{}

func New() core.Adapter { return &Adapter{} }

func freePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

func (a *Adapter) Start(inst *core.Instance, cfg []byte) error {
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
	inst.Handle = &handle{controllerURL: fmt.Sprintf("http://127.0.0.1:%d", port), secret: secret, port: port, config: cfg}
	current = inst
	return nil
}

func (a *Adapter) Stop(inst *core.Instance) error {
	mu.Lock()
	defer mu.Unlock()
	if current != inst {
		return core.ErrNotRunning
	}
	executor.Shutdown()
	current = nil
	inst.Handle = nil
	return nil
}

func (a *Adapter) Reload(inst *core.Instance, cfg []byte) error {
	if err := hub.Parse(cfg); err != nil {
		return err
	}
	if h, ok := inst.Handle.(*handle); ok {
		h.config = cfg
	}
	return nil
}

func (a *Adapter) Version() string { return "mihomo" }

func (a *Adapter) do(inst *core.Instance, method, path string, body any) (*http.Response, error) {
	h, _ := inst.Handle.(*handle)
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

func (a *Adapter) Proxies(inst *core.Instance) (map[string]any, error) {
	resp, err := a.do(inst, "GET", "/proxies", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return out, nil
}

func (a *Adapter) Delay(inst *core.Instance, name, testURL string, timeout time.Duration) (int, error) {
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

func (a *Adapter) SelectProxy(inst *core.Instance, group, name string) error {
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

func (a *Adapter) PatchConfig(inst *core.Instance, patch map[string]any) error {
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

func (a *Adapter) ReloadProviders(inst *core.Instance) error {
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

func (a *Adapter) SpeedTest(ctx context.Context, inst *core.Instance, proxyName string, downloadURL string, maxBytes int64, duration time.Duration) (float64, error) {
	speedTestMu.Lock()
	defer speedTestMu.Unlock()
	h, _ := inst.Handle.(*handle)
	if h == nil {
		return 0, core.ErrNotRunning
	}
	cfg, err := executor.ParseWithBytes(h.config)
	if err != nil {
		return 0, err
	}
	tempPort, err := freePort()
	if err != nil {
		return 0, err
	}
	temp, err := listener.ParseListener(map[string]any{
		"name":   "ohmycore-speedtest",
		"type":   "http",
		"listen": "127.0.0.1",
		"port":   tempPort,
		"proxy":  proxyName,
	})
	if err != nil {
		return 0, err
	}
	original := make(map[string]C.InboundListener, len(cfg.Listeners))
	for k, v := range cfg.Listeners {
		original[k] = v
	}
	combined := make(map[string]C.InboundListener, len(cfg.Listeners)+1)
	for k, v := range cfg.Listeners {
		combined[k] = v
	}
	combined[temp.Name()] = temp
	listener.PatchInboundListeners(combined, tunnel.Tunnel, true)
	defer listener.PatchInboundListeners(original, tunnel.Tunnel, true)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", tempPort), 200*time.Millisecond)
		if err == nil {
			conn.Close()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	proxyURL, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", tempPort))
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}, Timeout: duration}
	req, _ := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	start := time.Now()
	var total int64
	buf := make([]byte, 32*1024)
	for {
		if maxBytes > 0 && total >= maxBytes {
			break
		}
		if time.Since(start) >= duration {
			break
		}
		n, rerr := resp.Body.Read(buf)
		total += int64(n)
		if rerr != nil {
			if rerr == io.EOF {
				break
			}
			return 0, rerr
		}
	}
	elapsed := time.Since(start).Seconds()
	if elapsed <= 0 {
		return 0, nil
	}
	return float64(total) / elapsed, nil
}

var _ = listener.PatchInboundListeners
var _ = io.Discard
var _ = time.Second
var _ = http.StatusOK
