package service

import (
	"context"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"svc-registry/internal/apperr"
	"svc-registry/internal/catalog"
)

func ListBindings(ctx context.Context, state *State, p Principal, id uuid.UUID) ([]catalog.Binding, error) {
	scoped, err := Authorize(ctx, state, p, catalog.PermAccess, id)
	if err != nil {
		return nil, err
	}
	ids := []uuid.UUID{scoped.Node.ID}
	for i := len(scoped.Path) - 1; i >= 0; i-- {
		ids = append(ids, scoped.Path[i].Node.ID)
	}
	out, err := state.Bindings.List(ctx, ids)
	return out, apperr.Wrap(err)
}

func GrantRole(ctx context.Context, state *State, p Principal, id uuid.UUID, in catalog.GrantRole) (catalog.Binding, error) {
	if _, err := Authorize(ctx, state, p, catalog.PermAccess, id); err != nil {
		return catalog.Binding{}, err
	}
	kind, ok := catalog.ParseSubjectKind(in.SubjectKind)
	if !ok {
		return catalog.Binding{}, apperr.Wrap(catalog.InvalidSubjectKind)
	}
	role, ok := catalog.ParseRole(in.Role)
	if !ok {
		return catalog.Binding{}, apperr.Wrap(catalog.InvalidRole)
	}
	b, err := state.Bindings.Put(ctx, catalog.NewBinding{
		ID: uuid.Must(uuid.NewV7()), NodeID: id, SubjectKind: kind,
		Subject: strings.TrimFunc(in.Subject, unicode.IsSpace), Role: role,
	})
	if err != nil {
		return catalog.Binding{}, apperr.Wrap(err)
	}
	if b == nil {
		return catalog.Binding{}, apperr.Wrap(catalog.InvalidUnknownSubject)
	}
	return *b, nil
}

func RevokeBinding(ctx context.Context, state *State, p Principal, id, bindingID uuid.UUID) error {
	if _, err := Authorize(ctx, state, p, catalog.PermAccess, id); err != nil {
		return err
	}
	return apperr.Wrap(state.Bindings.Delete(ctx, id, bindingID))
}
