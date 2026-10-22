package requests

import catalogtypes "svc-registry/internal/feature/catalog/types"

type Secret struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Value       *string `json:"value"`
	Ref         *string `json:"ref"`
}

func (r Secret) Input() catalogtypes.SecretInput {
	return catalogtypes.SecretInput{Name: r.Name, Description: r.Description, Value: r.Value, Ref: r.Ref}
}

type Labels struct {
	Q   string  `query:"q"`
	Key *string `query:"key"`
}

func (r Labels) Query() catalogtypes.LabelQuery { return catalogtypes.LabelQuery{Q: r.Q, Key: r.Key} }
