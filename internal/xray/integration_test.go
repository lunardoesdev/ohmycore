//go:build integration

package xray

import (
	"os"
	"testing"
	"time"

	"ohmycore/core"
)

func TestXrayLifecycle(t *testing.T) {
	dir, _ := os.MkdirTemp("", "xray-int")
	defer os.RemoveAll(dir)
	m := core.NewManager(dir)
	m.RegisterAdapter(core.KernelXray, New())
	cfg, _ := os.ReadFile("../../testdata/xray.json")
	inst, err := m.Create(core.KernelXray, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Start(inst.ID); err != nil {
		t.Fatal(err)
	}
	if inst.Status != core.StatusRunning {
		t.Fatalf("status %s", inst.Status)
	}
	go func() { _ = m.Stop(inst.ID) }()
	time.Sleep(200 * time.Millisecond)
}
