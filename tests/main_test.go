package tests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	svcapp "svc-registry/internal/app"
)

var bin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "svc-registry-bin-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	bin = filepath.Join(dir, "svc-registry")
	build := exec.Command("go", "build", "-o", bin, "../cmd/svc-registry")
	build.Stdout, build.Stderr = os.Stderr, os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "build svc-registry:", err)
		os.Exit(1)
	}
	svcapp.DisableLogging()
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
