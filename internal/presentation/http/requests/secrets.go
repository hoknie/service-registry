package requests

import "svc-registry/internal/feature/catalog"

type Secret struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Value       *string `json:"value"`
	Ref         *string `json:"ref"`
}

func (r Secret) Input() catalog.SecretInput {
	return catalog.SecretInput{Name: r.Name, Description: r.Description, Value: r.Value, Ref: r.Ref}
}

type Labels struct {
	Q   string  `query:"q"`
	Key *string `query:"key"`
}

func (r Labels) Query() catalog.LabelQuery { return catalog.LabelQuery{Q: r.Q, Key: r.Key} }
