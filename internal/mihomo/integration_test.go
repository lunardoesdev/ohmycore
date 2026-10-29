//go:build integration

package mihomo

import (
	"os"
	"testing"
	"time"

	"ohmycore/core"
)

func TestMihomoLifecycle(t *testing.T) {
	dir, _ := os.MkdirTemp("", "mihomo-int")
	defer os.RemoveAll(dir)
	m := core.NewManager(dir)
	m.RegisterAdapter(core.KernelMihomo, New())
	cfg := []byte("mixed-port: 0\nmode: direct\nlog-level: error\n")
	inst, err := m.Create(core.KernelMihomo, cfg)
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
