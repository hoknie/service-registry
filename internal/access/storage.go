package access

import (
	"context"

	"github.com/google/uuid"
)

type UserStore interface {
	Insert(ctx context.Context, user NewUser) (User, error)
	InsertIfNone(ctx context.Context, user NewUser) (bool, error)
	Find(ctx context.Context, id uuid.UUID) (*User, error)
	FindCredentialsByEmail(ctx context.Context, email string) (*UserCredentials, error)
	FindCredentials(ctx context.Context, id uuid.UUID) (*UserCredentials, error)
	List(ctx context.Context, page PageRequest) (Page[User], error)
	Update(ctx context.Context, id uuid.UUID, changes UserChanges) (User, error)
	SetPassword(ctx context.Context, id uuid.UUID, passwordHash string) error
	ResetPassword(ctx context.Context, id uuid.UUID, passwordHash string) error
}

type SessionStore interface {
	Create(ctx context.Context, session NewSession) error
	FindValid(ctx context.Context, tokenHash [32]byte, idleTimeoutSecs uint64) (*SessionUser, error)
	DeleteByToken(ctx context.Context, tokenHash [32]byte) error
	DeleteForUser(ctx context.Context, userID uuid.UUID, keep *uuid.UUID) (uint64, error)
	DeleteExpiredForUser(ctx context.Context, userID uuid.UUID, idleTimeoutSecs uint64) (uint64, error)
}

type GroupStore interface {
	Insert(ctx context.Context, group NewGroup) (Group, error)
	List(ctx context.Context, page PageRequest) (Page[Group], error)
	Find(ctx context.Context, id uuid.UUID) (*GroupDetails, error)
	Rename(ctx context.Context, id uuid.UUID, name string) (Group, error)
	Delete(ctx context.Context, id uuid.UUID) error
	AddMember(ctx context.Context, id, groupID, userID uuid.UUID) error
	RemoveMember(ctx context.Context, groupID, userID uuid.UUID) error
	SyncManaged(ctx context.Context, userID uuid.UUID, source MembershipSource, groups []string) error
}

type IdentityStore interface {
	Find(ctx context.Context, provider, subject string) (*Identity, *User, error)
	Insert(ctx context.Context, identity NewIdentity) (Identity, error)
	Touch(ctx context.Context, id uuid.UUID, email *string) error
	ListForUser(ctx context.Context, userID uuid.UUID) ([]Identity, error)
	Delete(ctx context.Context, userID, id uuid.UUID) error
	CreateUser(ctx context.Context, user NewUser, identity NewIdentity) (User, error)
}

type LoginStateStore interface {
	Insert(ctx context.Context, state NewLoginState) error
	Take(ctx context.Context, browserHash [32]byte, state, provider string) (*LoginState, error)
	Prune(ctx context.Context) (int64, error)
}

type TokenStore interface {
	Insert(ctx context.Context, token NewToken) (Token, error)
	ListForUser(ctx context.Context, userID uuid.UUID) ([]Token, error)
	List(ctx context.Context, filter TokenFilter) (Page[Token], error)
	Revoke(ctx context.Context, id uuid.UUID, owner *uuid.UUID) error
	FindValid(ctx context.Context, tokenHash [32]byte) (*TokenUser, error)
}
