package types

import "svc-registry/internal/platform/apperr"

var (
	InvalidPublication = apperr.Invalid("validation.invalid_publication",
		`publication fields: publish, default_branch and current_release are true or false, versions is a list of {"pattern","kind":"branch"|"tag"}, name (folders and projects) is 1 to 63 of a-z, 0-9 and -, host is a lower-case host name`)
	ConflictNameTaken = apperr.Conflicting("conflict.publication_name_taken", "the public name is used by another node or an organization slug")
	ConflictHostTaken = apperr.Conflicting("conflict.publication_host_taken", "the host is used by another node")
)
