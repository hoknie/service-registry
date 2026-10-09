package response

import "svc-registry/internal/deploy"

type DirectoryEnvironment struct {
	Key       string            `json:"key"`
	Names     map[string]string `json:"names"`
	Position  int32             `json:"position"`
	CreatedAt string            `json:"created_at"`
	UpdatedAt string            `json:"updated_at"`
}

func DirectoryEnvironmentOf(e deploy.Environment) DirectoryEnvironment {
	return DirectoryEnvironment{Key: e.Key, Names: e.Names, Position: e.Position, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt}
}
