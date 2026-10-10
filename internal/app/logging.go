package app

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"svc-registry/internal/platform/config"
)

const LevelTrace = slog.LevelDebug - 4

func InitLogging(cfg config.LogConfig) {
	level, off := effectiveLevel(cfg.Filter)
	if off {
		slog.SetDefault(slog.New(discard{}))
		return
	}
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.LevelKey && a.Value.Any() == LevelTrace {
				return slog.String(slog.LevelKey, "TRACE")
			}
			return a
		},
	})
	slog.SetDefault(slog.New(h))
}

func DisableLogging() { slog.SetDefault(slog.New(discard{})) }

func effectiveLevel(filter string) (slog.Level, bool) {
	bare, verbose, seen := "", "", false
	rank := map[string]int{"off": 0, "error": 1, "warn": 2, "info": 3, "debug": 4, "trace": 5}
	for _, d := range strings.Split(filter, ",") {
		d = strings.TrimSpace(d)
		if i := strings.LastIndex(d, "="); i >= 0 {
			l := strings.TrimSpace(d[i+1:])
			if !seen || rank[l] > rank[verbose] {
				verbose, seen = l, true
			}
			continue
		}
		bare = d
	}
	l := bare
	if l == "" {
		l = verbose
	}
	switch l {
	case "error":
		return slog.LevelError, false
	case "warn":
		return slog.LevelWarn, false
	case "debug":
		return slog.LevelDebug, false
	case "trace":
		return LevelTrace, false
	case "off", "":
		return 0, true
	}
	return slog.LevelInfo, false
}

type discard struct{}

func (discard) Enabled(context.Context, slog.Level) bool  { return false }
func (discard) Handle(context.Context, slog.Record) error { return nil }
func (d discard) WithAttrs([]slog.Attr) slog.Handler      { return d }
func (d discard) WithGroup(string) slog.Handler           { return d }
