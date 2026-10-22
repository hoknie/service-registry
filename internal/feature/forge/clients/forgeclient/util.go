package forgeclient

import (
	"strings"
	"time"
)

var timeNow = time.Now

func splitFull(full string) (string, string) {
	i := strings.LastIndexByte(full, '/')
	if i < 0 {
		return "", full
	}
	return full[:i], full[i+1:]
}
