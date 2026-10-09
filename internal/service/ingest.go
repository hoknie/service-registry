package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"svc-registry/internal/apperr"
	"svc-registry/internal/auth"
	"svc-registry/internal/ingest"
)

type Authenticated struct {
	projectID uuid.UUID
	keyID     uuid.UUID
}

func AuthenticateIngest(ctx context.Context, state *State, projectID uuid.UUID, bearer *string) (Authenticated, error) {
	if bearer == nil {
		return Authenticated{}, apperr.New(apperr.InvalidProjectKey)
	}
	keyID, err := auth.VerifyProjectKey(ctx, state.ProjectKeys, projectID, *bearer)
	if err != nil {
		return Authenticated{}, apperr.Wrap(err)
	}
	if keyID == nil {
		return Authenticated{}, apperr.New(apperr.InvalidProjectKey)
	}
	if secs, ok := state.IngestLimiter.Hit(*keyID, time.Now()); !ok {
		return Authenticated{}, &apperr.Error{Kind: apperr.IngestRateLimited, RetryAfterSecs: secs}
	}
	return Authenticated{projectID: projectID, keyID: *keyID}, nil
}

func AcceptEvent(ctx context.Context, state *State, who Authenticated, body ingest.WireEnvelope) (ingest.Accepted, error) {
	envelope, err := ingest.ParseEnvelope(body)
	if err != nil {
		return ingest.Accepted{}, apperr.Wrap(err)
	}
	accepted, err := state.Events.Accept(ctx, ingest.NewEvent{
		ID: uuid.Must(uuid.NewV7()), ProjectID: who.projectID, KeyID: who.keyID,
		DeploymentID: uuid.Must(uuid.NewV7()), EnvironmentID: uuid.Must(uuid.NewV7()),
		Envelope: envelope,
	})
	return accepted, apperr.Wrap(err)
}

func PruneEvents(ctx context.Context, state *State) (uint64, error) {
	n, err := state.Events.Prune(ctx, state.Config.Ingest.RetentionDays)
	return n, apperr.Wrap(err)
}
