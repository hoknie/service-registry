package types

import "svc-registry/internal/platform/apperr"

var InvalidOwners = apperr.Invalid("validation.invalid_owners", "owners must be at most 10 distinct users or groups")
