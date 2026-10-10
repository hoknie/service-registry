package config

import "net/netip"

type HTTPConfig struct {
	Addr                netip.AddrPort `env:"HTTP_ADDR" envDefault:"0.0.0.0:8080"`
	ShutdownTimeoutSecs uint64         `env:"SHUTDOWN_TIMEOUT_SECS" envDefault:"10" validate:"max=300"`
}
