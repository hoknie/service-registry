package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type KnowledgeConfig struct {
	IntervalSecs       uint32   `env:"KNOWLEDGE_INTERVAL_SECS" envDefault:"300" validate:"min=30,max=86400"`
	RetrySecs          uint32   `env:"KNOWLEDGE_RETRY_SECS" envDefault:"900" validate:"min=60,max=86400"`
	CollectConcurrency uint32   `env:"KNOWLEDGE_COLLECT_CONCURRENCY" envDefault:"2" validate:"min=1,max=16"`
	Keep               uint32   `env:"KNOWLEDGE_KEEP" envDefault:"5" validate:"max=100"`
	ScanHistory        uint32   `env:"KNOWLEDGE_SCAN_HISTORY" envDefault:"20" validate:"min=1,max=1000"`
	MaxFileBytes       int64    `env:"KNOWLEDGE_MAX_FILE_BYTES" envDefault:"524288" validate:"min=1024,max=10485760"`
	MaxFiles           uint32   `env:"KNOWLEDGE_MAX_FILES" envDefault:"2000" validate:"min=1,max=50000"`
	MaxSnapshotBytes   int64    `env:"KNOWLEDGE_MAX_SNAPSHOT_BYTES" envDefault:"16777216" validate:"min=65536,max=268435456"`
	MCPMaxResultBytes  int      `env:"KNOWLEDGE_MCP_MAX_RESULT_BYTES" envDefault:"262144" validate:"min=4096,max=4194304"`
	LocalRootsRaw      string   `env:"KNOWLEDGE_LOCAL_ROOTS"`
	LocalRoots         []string `env:"-"`
}

func (c *KnowledgeConfig) check() Errors {
	c.LocalRoots = nil
	for _, raw := range strings.Split(c.LocalRootsRaw, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if !filepath.IsAbs(raw) {
			return Errors{{Var: "KNOWLEDGE_LOCAL_ROOTS", Reason: fmt.Sprintf("%q must be an absolute path", raw)}}
		}
		resolved, err := filepath.EvalSymlinks(raw)
		if err != nil {
			return Errors{{Var: "KNOWLEDGE_LOCAL_ROOTS", Reason: fmt.Sprintf("%q does not exist", raw)}}
		}
		if st, err := os.Stat(resolved); err != nil || !st.IsDir() {
			return Errors{{Var: "KNOWLEDGE_LOCAL_ROOTS", Reason: fmt.Sprintf("%q is not a directory", raw)}}
		}
		c.LocalRoots = append(c.LocalRoots, resolved)
	}
	return nil
}
