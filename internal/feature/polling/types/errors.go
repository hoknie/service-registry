package types

import "svc-registry/internal/platform/apperr"

var InvalidPolling = apperr.Invalid("validation.invalid_polling",
	"polling fields: interval_secs 60..=86400, timeout_secs 1..=600, mode on, off or published; null inherits")
