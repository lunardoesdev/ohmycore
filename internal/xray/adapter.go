package xray

import (
	_ "github.com/xtls/xray-core/app/proxyman/inbound"
	_ "github.com/xtls/xray-core/app/proxyman/outbound"
	_ "github.com/xtls/xray-core/main/json"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"time"

	xcore "github.com/xtls/xray-core/core"

	"ohmycore/core"
)

type Adapter struct{}

func New() core.Adapter { return &Adapter{} }

func (a *Adapter) Start(inst *core.Instance, cfg []byte) error {
	c, err := xcore.LoadConfig("json", bytes.NewReader(cfg))
	if err != nil {
		return err
	}
	srv, err := xcore.New(c)
	if err != nil {
		return err
	}
	go func() { _ = srv.Start() }()
	inst.Handle = srv
	return nil
}

func (a *Adapter) Stop(inst *core.Instance) error {
	h := inst.Handle
	inst.Handle = nil
	if h == nil {
		return nil
	}
	done := make(chan error, 1)
	go func() { done <- h.(*xcore.Instance).Close() }()
	select {
	case err := <-done:
		return err
	case <-time.After(3 * time.Second):
		return nil
	}
}

func (a *Adapter) Reload(inst *core.Instance, cfg []byte) error {
	if err := a.Stop(inst); err != nil {
		return err
	}
	return a.Start(inst, cfg)
}

func (a *Adapter) Version() string { return xcore.Version() }

func (a *Adapter) Ping(ctx context.Context, cfg []byte, outboundTag, testURL string, timeout time.Duration) (int64, error) {
	srv, err := tempInstance(cfg)
	if err != nil {
		return 0, err
	}
	defer srv.Close()
	start := time.Now()
	req, _ := http.NewRequestWithContext(ctx, "GET", testURL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	_ = io.Discard
	return time.Since(start).Milliseconds(), nil
}

func (a *Adapter) SpeedTest(ctx context.Context, cfg []byte, outboundTag, downloadURL string, duration time.Duration) (float64, error) {
	srv, err := tempInstance(cfg)
	if err != nil {
		return 0, err
	}
	defer srv.Close()
	proxyURL, _ := url.Parse("socks5://127.0.0.1:10808")
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

func tempInstance(cfg []byte) (*xcore.Instance, error) {
	c, err := xcore.LoadConfig("json", bytes.NewReader(cfg))
	if err != nil {
		return nil, err
	}
	srv, err := xcore.New(c)
	if err != nil {
		return nil, err
	}
	go func() { _ = srv.Start() }()
	time.Sleep(200 * time.Millisecond)
	return srv, nil
}
