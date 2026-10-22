package types

type Access struct {
	InCluster bool
	APIURL    string
	CAPEM     []byte
	Token     string
}
