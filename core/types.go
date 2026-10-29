package core

import (
	"errors"
	"sync"
	"time"
)

// Kernel identifies a supported proxy backend.
type Kernel string

const (
	KernelXray   Kernel = "xray"
	KernelMihomo Kernel = "mihomo"
)

// Status represents the lifecycle state of an instance.
type Status string

const (
	StatusCreated Status = "created"
	StatusRunning Status = "running"
	StatusStopped Status = "stopped"
	StatusError   Status = "error"
)

// Instance is the persisted public representation of a proxy instance.
type Instance struct {
	ID        string    `json:"id"`
	Kernel    Kernel    `json:"kernel"`
	Status    Status    `json:"status"`
	WorkDir   string    `json:"work_dir"`
	StartedAt time.Time `json:"started_at,omitempty"`
	LastError string    `json:"last_error,omitempty"`

	// exported for internal adapter access
	Mu     sync.RWMutex `json:"-"`
	Handle any          `json:"-"`
}

// Node is a normalized proxy outbound.
type Node struct {
	Tag       string         `json:"tag"`
	Protocol  string         `json:"protocol"`
	Server    string         `json:"server"`
	Port      int            `json:"port"`
	UUID      string         `json:"uuid,omitempty"`
	Password  string         `json:"password,omitempty"`
	Transport string         `json:"transport,omitempty"`
	TLS       bool           `json:"tls,omitempty"`
	SNI       string         `json:"sni,omitempty"`
	Extra     map[string]any `json:"extra,omitempty"`
}

// Manager coordinates instances.
type Manager struct {
	baseDir   string
	mu        sync.RWMutex
	instances map[string]*Instance
	adapters  map[Kernel]Adapter
}

// NewManager creates a manager rooted at baseDir.
func NewManager(baseDir string) *Manager {
	return &Manager{
		baseDir:   baseDir,
		instances: make(map[string]*Instance),
		adapters:  make(map[Kernel]Adapter),
	}
}

// Adapter abstracts a backend implementation.
type Adapter interface {
	Start(inst *Instance, cfg []byte) error
	Stop(inst *Instance) error
	Reload(inst *Instance, cfg []byte) error
	Version() string
}

var (
	ErrNotFound        = errors.New("instance not found")
	ErrAlreadyRunning  = errors.New("instance already running")
	ErrNotRunning      = errors.New("instance not running")
	ErrMihomoBusy      = errors.New("mihomo instance already running in this process")
	ErrProxyInUse      = errors.New("proxy is in use")
	ErrUnsupportedLink = errors.New("unsupported link")
)
