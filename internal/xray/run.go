package xray

import (
	"bytes"
	"fmt"

	xcore "github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/infra/conf/serial"

	// Register all inbound/outbound/transport/app implementations.
	_ "github.com/xtls/xray-core/main/distro/all"

	// Hysteria2 ships with xray-core but is left out of distro/all, so it is
	// registered here: subscriptions routinely include hysteria servers.
	_ "github.com/xtls/xray-core/proxy/hysteria"
	_ "github.com/xtls/xray-core/transport/internet/hysteria"
)

// Instance is a running embedded xray-core instance.
type Instance struct {
	inst *xcore.Instance
}

// New loads a JSON config and builds an embedded xray-core instance without
// starting it. Creating the instance registers xray's own log handler, which
// writes raw lines to stdout from a background goroutine; the gap between New
// and Start is where a caller installs its own handler (see internal/xraylog),
// so engine output is rendered by us and ordered against our own printing.
func New(configJSON []byte) (*Instance, error) {
	cfg, err := serial.LoadJSONConfig(bytes.NewReader(configJSON))
	if err != nil {
		return nil, fmt.Errorf("xray: load config: %w", err)
	}
	inst, err := xcore.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("xray: create instance: %w", err)
	}
	return &Instance{inst: inst}, nil
}

// Start starts the instance. It keeps running until Close is called.
func (i *Instance) Start() error {
	if err := i.inst.Start(); err != nil {
		return fmt.Errorf("xray: start instance: %w", err)
	}
	return nil
}

// Close stops the instance and releases its resources.
func (i *Instance) Close() error {
	if i == nil || i.inst == nil {
		return nil
	}
	return i.inst.Close()
}
