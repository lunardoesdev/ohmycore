package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// RegisterAdapter registers a backend adapter for a kernel.
func (m *Manager) RegisterAdapter(kernel Kernel, a Adapter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.adapters[kernel] = a
}

func (m *Manager) adapter(kernel Kernel) (Adapter, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.adapters[kernel]
	if !ok {
		return nil, fmt.Errorf("no adapter registered for %s", kernel)
	}
	return a, nil
}

// Create allocates a new instance and persists its config.
func (m *Manager) Create(kernel Kernel, config []byte) (*Instance, error) {
	if kernel != KernelXray && kernel != KernelMihomo {
		return nil, fmt.Errorf("unknown kernel %s", kernel)
	}
	id := uuid.NewString()
	inst := &Instance{
		ID:     id,
		Kernel: kernel,
		Status: StatusCreated,
	}
	inst.WorkDir = filepath.Join(m.baseDir, "instances", id)
	if err := os.MkdirAll(inst.WorkDir, 0o700); err != nil {
		return nil, err
	}
	ext := "json"
	if kernel == KernelMihomo {
		ext = "yaml"
	}
	cfgPath := filepath.Join(inst.WorkDir, "config."+ext)
	if err := os.WriteFile(cfgPath, config, 0o600); err != nil {
		os.RemoveAll(inst.WorkDir)
		return nil, err
	}
	m.mu.Lock()
	m.instances[id] = inst
	m.mu.Unlock()
	return inst, nil
}

// Get returns an instance by id.
func (m *Manager) Get(id string) (*Instance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	inst, ok := m.instances[id]
	if !ok {
		return nil, ErrNotFound
	}
	return inst, nil
}

// List returns all instances.
func (m *Manager) List() []*Instance {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Instance, 0, len(m.instances))
	for _, inst := range m.instances {
		out = append(out, inst)
	}
	return out
}

// Start starts an instance.
func (m *Manager) Start(id string) error {
	inst, err := m.Get(id)
	if err != nil {
		return err
	}
	inst.Mu.Lock()
	defer inst.Mu.Unlock()
	if inst.Status == StatusRunning {
		return ErrAlreadyRunning
	}
	a, err := m.adapter(inst.Kernel)
	if err != nil {
		return err
	}
	cfgPath := configPath(inst)
	cfg, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	if err := a.Start(inst, cfg); err != nil {
		inst.Status = StatusError
		inst.LastError = err.Error()
		return err
	}
	inst.Status = StatusRunning
	inst.StartedAt = time.Now()
	inst.LastError = ""
	return nil
}

// Stop stops an instance.
func (m *Manager) Stop(id string) error {
	inst, err := m.Get(id)
	if err != nil {
		return err
	}
	inst.Mu.Lock()
	defer inst.Mu.Unlock()
	if inst.Status != StatusRunning {
		return ErrNotRunning
	}
	a, err := m.adapter(inst.Kernel)
	if err != nil {
		return err
	}
	if err := a.Stop(inst); err != nil {
		inst.Status = StatusError
		inst.LastError = err.Error()
		return err
	}
	inst.Status = StatusStopped
	inst.LastError = ""
	return nil
}

// Reload rewrites config and restarts (Xray) or applies (Mihomo).
func (m *Manager) Reload(id string, config []byte) error {
	inst, err := m.Get(id)
	if err != nil {
		return err
	}
	inst.Mu.Lock()
	defer inst.Mu.Unlock()
	ext := "json"
	if inst.Kernel == KernelMihomo {
		ext = "yaml"
	}
	cfgPath := filepath.Join(inst.WorkDir, "config."+ext)
	if err := os.WriteFile(cfgPath, config, 0o600); err != nil {
		return err
	}
	a, err := m.adapter(inst.Kernel)
	if err != nil {
		return err
	}
	if inst.Status == StatusRunning {
		if err := a.Reload(inst, config); err != nil {
			inst.Status = StatusError
			inst.LastError = err.Error()
			return err
		}
	}
	inst.LastError = ""
	return nil
}

// Remove stops (if running) and deletes an instance.
func (m *Manager) Remove(id string) error {
	inst, err := m.Get(id)
	if err != nil {
		return err
	}
	inst.Mu.Lock()
	if inst.Status == StatusRunning {
		a, aerr := m.adapter(inst.Kernel)
		if aerr == nil {
			_ = a.Stop(inst)
		}
	}
	inst.Mu.Unlock()
	m.mu.Lock()
	delete(m.instances, id)
	m.mu.Unlock()
	return os.RemoveAll(inst.WorkDir)
}

// Close stops all running instances.
func (m *Manager) Close() {
	for _, inst := range m.List() {
		if inst.Status == StatusRunning {
			_ = m.Stop(inst.ID)
		}
	}
}

func configPath(inst *Instance) string {
	ext := "json"
	if inst.Kernel == KernelMihomo {
		ext = "yaml"
	}
	return filepath.Join(inst.WorkDir, "config."+ext)
}

// MarshalJSON for Instance (handle excluded).
func (i *Instance) MarshalJSON() ([]byte, error) {
	i.Mu.RLock()
	defer i.Mu.RUnlock()
	type alias struct {
		ID        string    `json:"id"`
		Kernel    Kernel    `json:"kernel"`
		Status    Status    `json:"status"`
		WorkDir   string    `json:"work_dir"`
		StartedAt time.Time `json:"started_at,omitempty"`
		LastError string    `json:"last_error,omitempty"`
	}
	return json.Marshal(alias{
		ID:        i.ID,
		Kernel:    i.Kernel,
		Status:    i.Status,
		WorkDir:   i.WorkDir,
		StartedAt: i.StartedAt,
		LastError: i.LastError,
	})
}

var _ sync.Locker = (*sync.Mutex)(nil)
