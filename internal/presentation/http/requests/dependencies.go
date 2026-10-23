package requests

type Dependency struct {
	Target *string `json:"target" validate:"required"`
	Note   *string `json:"note"`
}

type DependencyGraphQuery struct {
	Depth *int `query:"depth"`
}
