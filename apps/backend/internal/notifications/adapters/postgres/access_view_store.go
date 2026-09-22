package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	identitypg "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// AccessEventUserReader is the subset of the identity user repository the
// access events' view resolution needs. Declared here so this adapter depends
// on a narrow contract rather than the full identity repository — the same
// shape the access context's own user lookup adapter takes.
type AccessEventUserReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (AccessEventUser, error)
}

// AccessEventUser is the display-relevant projection of a registered user.
type AccessEventUser struct {
	Name    *string
	Surname *string
	Phone   string
	Email   string
}

// AccessViewStore answers the access events' display questions (#751) over
// the owning tables: the property snapshot from properties, the user profile
// over the identity user repository. Read-only — the events publish through
// the pipeline, nothing is written here.
type AccessViewStore struct {
	queries *postgres.Queries
	users   AccessEventUserReader
}

// NewAccessViewStore creates the access events' view store over the pool and
// the identity user reader.
func NewAccessViewStore(db postgres.DBTX, users AccessEventUserReader) *AccessViewStore {
	return &AccessViewStore{queries: postgres.New(db), users: users}
}

// accessEventUserAdapter adapts the identity user repository to the narrow
// reader — the same bridge shape the access context's user lookup takes.
type accessEventUserAdapter struct {
	users *identitypg.UserRepository
}

// NewAccessEventUserReader adapts the identity user repository.
func NewAccessEventUserReader(users *identitypg.UserRepository) AccessEventUserReader {
	return &accessEventUserAdapter{users: users}
}

func (a *accessEventUserAdapter) GetByID(ctx context.Context, id uuid.UUID) (AccessEventUser, error) {
	u, err := a.users.GetByID(ctx, id)
	if err != nil {
		return AccessEventUser{}, err
	}
	var email string
	if u.Email != nil {
		email = u.Email.String()
	}
	return AccessEventUser{Name: u.Name, Surname: u.Surname, Phone: u.Phone.String(), Email: email}, nil
}

// The adapter satisfies the consumer-declared port (CODING_STANDARDS).
var _ application.AccessEventViewSource = (*AccessViewStore)(nil)

// PropertyView returns the property's display snapshot: the name and the
// address line the feed rows' property card carries.
func (s *AccessViewStore) PropertyView(ctx context.Context, propertyID uuid.UUID) (application.AccessPropertyView, error) {
	row, err := s.queries.GetAccessEventPropertyView(ctx, pgconv.UUIDToPgtype(propertyID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.AccessPropertyView{}, fmt.Errorf("property %s not found: %w", propertyID, application.ErrNotFound)
		}
		return application.AccessPropertyView{}, fmt.Errorf("get property view %s: %w", propertyID, err)
	}
	return application.AccessPropertyView{Name: row.Name, Address: row.Address}, nil
}

// UserProfileView returns the user's display snapshot: the display name (the
// access display-name canon — the profile's name, or the masked phone when
// the profile has none; the raw phone and the email are never a display name)
// and the email the actor card shows. The copy is this context's own
// rendering of the shared rule (per-context canon, as the scan publishers'
// date rendering) — the access context keeps its own.
func (s *AccessViewStore) UserProfileView(ctx context.Context, userID uuid.UUID) (application.AccessUserProfile, error) {
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return application.AccessUserProfile{}, fmt.Errorf("get user profile %s: %w", userID, err)
	}
	return application.AccessUserProfile{
		DisplayName: accessEventDisplayName(u.Name, u.Surname, u.Phone),
		Email:       u.Email,
	}, nil
}

// accessEventDisplayName joins the profile name ("Name Surname") with the
// masked-phone fallback — a display name never reveals a full phone or an
// email.
func accessEventDisplayName(name, surname *string, phone string) string {
	parts := make([]string, 0, 2)
	if name != nil && *name != "" {
		parts = append(parts, *name)
	}
	if surname != nil && *surname != "" {
		parts = append(parts, *surname)
	}
	if joined := strings.Join(parts, " "); joined != "" {
		return joined
	}
	return accessEventMaskPhone(phone)
}

// accessEventMaskPhone masks all but the country code and the last two digits
// of the phone.
func accessEventMaskPhone(phone string) string {
	if len(phone) <= 4 {
		return phone
	}
	return phone[:2] + strings.Repeat("*", len(phone)-4) + phone[len(phone)-2:]
}
