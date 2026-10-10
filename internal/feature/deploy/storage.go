package deploy

import (
	"github.com/google/uuid"
)

type StoredSecret struct {
	ID  uuid.UUID
	Enc string
}

type ProjectRef struct {
	ID      uuid.UUID
	Observe bool
}

type StoredCredentials struct {
	SecretID *uuid.UUID
	Enc      *string
	Ref      *string
}
