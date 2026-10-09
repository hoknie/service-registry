package webui

import (
	"fmt"
	"time"
)

func cacheControl(segments []string) string {
	if len(segments) >= 2 && segments[0] == "_next" && segments[1] == "static" {
		return "public, max-age=31536000, immutable"
	}
	return "no-cache"
}

func etag(size int64, modified time.Time, encoding string) string {
	nanos := modified.UnixNano()
	if nanos < 0 || modified.IsZero() {
		nanos = 0
	}
	switch encoding {
	case "br":
		return fmt.Sprintf("\"%x-%x-br\"", size, nanos)
	case "gzip":
		return fmt.Sprintf("\"%x-%x-gz\"", size, nanos)
	}
	return fmt.Sprintf("\"%x-%x\"", size, nanos)
}
