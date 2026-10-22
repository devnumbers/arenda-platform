package wire

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	accessapp "github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	accessdomain "github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	historyapp "github.com/nambers/arenda-planform/apps/backend/internal/history/application"
)

// The participant-scope bridge's unit seam (карта #1582, тикет #1583): the
// participants manager fake pins the mapping — the aggregate's legs become
// the feed scope, and the access not-found sentinel becomes the history
// privacy sentinel (ADR 0035: the glue is the only place both vocabularies
// meet).

type participantsManagerFake struct {
	participant accessapp.Participant
	err         error
}

func (f participantsManagerFake) ListParticipants(context.Context, uuid.UUID) ([]accessapp.Participant, error) {
	return nil, errors.New("unexpected ListParticipants call")
}

func (f participantsManagerFake) GetParticipant(context.Context, uuid.UUID, string) (accessapp.Participant, error) {
	return f.participant, f.err
}

func TestParticipantScopeBridgeMapsAggregate(t *testing.T) {
	t.Parallel()

	userID := uuid.Must(uuid.NewV7())
	leg1, leg2 := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	bridge := newParticipantScopeBridge(participantsManagerFake{
		participant: accessapp.Participant{
			UserID: userID,
			Properties: []accessapp.ParticipantProperty{
				{PropertyID: leg1},
				{PropertyID: leg2},
			},
		},
	})

	scope, err := bridge.ResolveParticipantScope(t.Context(), uuid.Must(uuid.NewV7()), userID.String())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if scope.UserID != userID {
		t.Fatalf("user id = %s, want %s", scope.UserID, userID)
	}
	if len(scope.PropertyIDs) != 2 || scope.PropertyIDs[0] != leg1 || scope.PropertyIDs[1] != leg2 {
		t.Fatalf("property ids = %+v, want the legs in order", scope.PropertyIDs)
	}
}

func TestParticipantScopeBridgeMapsPendingZeroUser(t *testing.T) {
	t.Parallel()

	leg := uuid.Must(uuid.NewV7())
	bridge := newParticipantScopeBridge(participantsManagerFake{
		participant: accessapp.Participant{
			Email:      "pending@example.com",
			Properties: []accessapp.ParticipantProperty{{PropertyID: leg}},
		},
	})

	scope, err := bridge.ResolveParticipantScope(t.Context(), uuid.Must(uuid.NewV7()), "pending@example.com")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if scope.UserID != (uuid.UUID{}) {
		t.Fatalf("pending user id = %s, want zero", scope.UserID)
	}
	if len(scope.PropertyIDs) != 1 || scope.PropertyIDs[0] != leg {
		t.Fatalf("property ids = %+v, want the pending leg", scope.PropertyIDs)
	}
}

func TestParticipantScopeBridgeMapsPrivacySentinel(t *testing.T) {
	t.Parallel()

	const stranger = "stranger@example.com"
	bridge := newParticipantScopeBridge(participantsManagerFake{
		err: accessdomain.ErrParticipantNotFound,
	})

	_, err := bridge.ResolveParticipantScope(t.Context(), uuid.Must(uuid.NewV7()), stranger)
	if !errors.Is(err, historyapp.ErrNotFound) {
		t.Fatalf("stranger: want historyapp.ErrNotFound, got %v", err)
	}

	// Чужой сбой хранилища — не приватность: наверх как есть (500 обработчика).
	down := newParticipantScopeBridge(participantsManagerFake{err: errors.New("store down")})
	_, err = down.ResolveParticipantScope(t.Context(), uuid.Must(uuid.NewV7()), "x@example.com")
	if err == nil || errors.Is(err, historyapp.ErrNotFound) {
		t.Fatalf("store failure: want a propagated error, got %v", err)
	}
}
