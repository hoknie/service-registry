package types

import "time"

const MaxIconBytes = 64 << 10

type IconFile struct {
	Name     string
	Upload   bool
	Modified time.Time
}
