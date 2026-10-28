package types

import "github.com/google/uuid"

const (
	OutcomeOK     = "ok"
	OutcomeFailed = "failed"
)

type Check struct {
	At         string
	Outcome    string
	Code       *string
	Detail     *string
	DurationMs int
	Truncated  bool
}

type IconFile struct {
	Type string
	Data []byte
}

type Result struct {
	Info      ServerInfo
	Tools     []Tool
	Truncated bool
	Icon      *IconFile
}

type CheckRecord struct {
	ServerID   uuid.UUID
	Outcome    string
	Code       *string
	Detail     *string
	DurationMs int
	Truncated  bool
	Info       *ServerInfo
	Tools      []Tool
	Icon       *IconFile
}

func CheckOf(server uuid.UUID, res *Result, err error, durationMs int, secrets ...string) CheckRecord {
	rec := CheckRecord{ServerID: server, DurationMs: durationMs}
	if err != nil {
		code, detail := FailureCode(err), Redact(FailureDetail(err), secrets...)
		rec.Outcome, rec.Code, rec.Detail = OutcomeFailed, &code, &detail
		return rec
	}
	info := res.Info
	info.Instructions = truncate(info.Instructions, 4000)
	rec.Outcome, rec.Info, rec.Tools, rec.Truncated, rec.Icon = OutcomeOK, &info, res.Tools, res.Truncated, res.Icon
	if rec.Tools == nil {
		rec.Tools = []Tool{}
	}
	return rec
}
