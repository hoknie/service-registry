package requests

import ownershiptypes "svc-registry/internal/feature/ownership/types"

type Owner struct {
	SubjectKind *string `json:"subject_kind" validate:"required"`
	Subject     *string `json:"subject" validate:"required"`
}

type Owners struct {
	Owners *[]Owner `json:"owners" validate:"required,dive"`
	None   bool     `json:"none"`
}

func (r Owners) Set() []ownershiptypes.PutOwner {
	out := make([]ownershiptypes.PutOwner, 0, len(*r.Owners))
	for _, o := range *r.Owners {
		out = append(out, ownershiptypes.PutOwner{SubjectKind: *o.SubjectKind, Subject: *o.Subject})
	}
	return out
}

type OwnerQuery struct {
	Q string `query:"q"`
}
