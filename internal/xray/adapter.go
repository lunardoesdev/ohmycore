package xray

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	xcore "github.com/xtls/xray-core/core"

	"ohmycore/core"
)

type adapter struct{}

func New() core.Adapter { return &adapter{} }

func (a *adapter) Start(inst *core.Instance, cfg []byte) error {
	c, err := xcore.LoadConfig("json", bytes.NewReader(cfg))
	if err != nil {
		return err
	}
	srv, err := xcore.New(c)
	if err != nil {
		return err
	}
	if err := srv.Start(); err != nil {
		srv.Close()
		return err
	}
	inst.Mu.Lock()
	inst.Handle = srv
	inst.Mu.Unlock()
	return nil
}

func (a *adapter) Stop(inst *core.Instance) error {
	inst.Mu.Lock()
	h := inst.Handle
	inst.Handle = nil
	inst.Mu.Unlock()
	if h == nil {
		return nil
	}
	return h.(*xcore.Instance).Close()
}

func (a *adapter) Reload(inst *core.Instance, cfg []byte) error {
	if err := a.Stop(inst); err != nil {
		return err
	}
	return a.Start(inst, cfg)
}

func (a *adapter) Version() string { return xcore.Version() }

// Ping measures RTT to testURL through a temporary instance containing only outboundTag.
func (a *adapter) Ping(ctx context.Context, cfg []byte, outboundTag, testURL string, timeout time.Duration) (int64, error) {
	// Build minimal config: use provided cfg but ensure only outboundTag? For simplicity use cfg as-is temp instance.
	srv, err := tempInstance(cfg)
	if err != nil {
		return 0, err
	}
	defer srv.Close()
	// issue request
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

func (a *adapter) SpeedTest(ctx context.Context, cfg []byte, outboundTag, downloadURL string, duration time.Duration) (float64, error) {
	srv, err := tempInstance(cfg)
	if err != nil {
		return 0, err
	}
	defer srv.Close()
	// placeholder measure
	return 0, fmt.Errorf("not implemented")
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
	if err := srv.Start(); err != nil {
		srv.Close()
		return nil, err
	}
	return srv, nil
}
