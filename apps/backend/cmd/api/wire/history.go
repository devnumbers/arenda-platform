package wire

import (
	"context"
	"errors"

	"github.com/google/uuid"
	accessapp "github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	accessdomain "github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	billinghttp "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/http"
	historypg "github.com/nambers/arenda-planform/apps/backend/internal/history/adapters/postgres"
	historyapp "github.com/nambers/arenda-planform/apps/backend/internal/history/application"
)

// HistoryReadMod is the history module's reading side (карта #704, тикет
// #708, ADR 0061 §7): the feed page and the filter-sheet options.
type HistoryReadMod struct {
	Service *historyapp.HistoryReadService
}

// The billing paid-sections gate adapter doubles as the journal's
// tariff-gate port (ADR 0064, ревизия #1465): one adapter, three consumer-
// side ports — the middleware's, the access use cases' and the history
// reads'. This assertion lives in the wiring layer (the only place allowed
// to depend on several bounded contexts at once).
var _ historyapp.PaidSectionsGate = (*billinghttp.PaidSectionsGate)(nil)

// WireHistoryRead builds the action journal's reading service: the read
// store plus the membership policy, the role-aware tariff gate and the
// participant-scope bridge. Must run after the policy is installed
// (p.Policy = accessMod.Policy) — the property_ids scope proof resolves
// roles through it; on the owner-only fallback every member's scoped
// request would privacy-404.
func WireHistoryRead(p platformDeps, billing *Billing, participants *accessapp.ParticipantService) HistoryReadMod {
	return HistoryReadMod{
		Service: historyapp.NewHistoryReadService(
			historypg.NewReadStore(p.DB), p.Policy, billing.PaidSectionsGate,
			newParticipantScopeBridge(participants),
		),
	}
}

// participantScopeBridge fits the access participants service to the
// journal feed's participant-scope port (карта #1582, тикет #1583, ADR
// 0035): the «Действия участника» page's scope — the person's user uuid and
// the property ids of their current access legs — is the aggregate GET
// /participants/{participantId} already returns, resolved inside the
// reader's scope (owner-only since D8, #1304); the glue lives here so the
// history application never imports access. A person outside the reader's
// scope is access's ErrParticipantNotFound, mapped onto the history
// sentinel — the privacy 404 of the whole request (канон #547). The feed's
// scope follows the aggregate's legs (неархивные объекты читающего — как у
// самой страницы участника): отозванная нога уводит свои строки со
// страницы, pending-приглашение остаётся ногой.
type participantScopeBridge struct {
	participants accessapp.ParticipantsManager
}

func newParticipantScopeBridge(participants accessapp.ParticipantsManager) participantScopeBridge {
	return participantScopeBridge{participants: participants}
}

var _ historyapp.ParticipantScopeResolver = participantScopeBridge{}

func (b participantScopeBridge) ResolveParticipantScope(
	ctx context.Context, actor uuid.UUID, participantID string,
) (historyapp.ParticipantScope, error) {
	participant, err := b.participants.GetParticipant(ctx, actor, participantID)
	if err != nil {
		if errors.Is(err, accessdomain.ErrParticipantNotFound) {
			return historyapp.ParticipantScope{}, historyapp.ErrNotFound
		}
		return historyapp.ParticipantScope{}, err
	}
	ids := make([]uuid.UUID, 0, len(participant.Properties))
	for _, leg := range participant.Properties {
		ids = append(ids, leg.PropertyID)
	}
	return historyapp.ParticipantScope{UserID: participant.UserID, PropertyIDs: ids}, nil
}
