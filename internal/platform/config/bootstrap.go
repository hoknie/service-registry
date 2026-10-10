package config

import "fmt"

type BootstrapConfig struct {
	Admin *BootstrapAdmin `env:",init" envPrefix:"BOOTSTRAP_ADMIN_"`
}

func (c *BootstrapConfig) check() Errors {
	a := c.Admin
	switch {
	case a == nil || (a.Email == "" && a.Password == ""):
		c.Admin = nil
	case a.Password == "":
		return Errors{{Var: "BOOTSTRAP_ADMIN_PASSWORD", Reason: "is required when BOOTSTRAP_ADMIN_EMAIL is set"}}
	case a.Email == "":
		return Errors{{Var: "BOOTSTRAP_ADMIN_EMAIL", Reason: "is required when BOOTSTRAP_ADMIN_PASSWORD is set"}}
	}
	return nil
}

type BootstrapAdmin struct {
	Email    string `env:"EMAIL"`
	Password string `env:"PASSWORD"`
}

func (a BootstrapAdmin) String() string {
	return fmt.Sprintf("BootstrapAdmin{Email: %q, Password: <redacted>}", a.Email)
}

func (a BootstrapAdmin) GoString() string { return a.String() }
