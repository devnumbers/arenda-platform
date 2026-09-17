package postgres

import (
	"context"
	"fmt"
	"math"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/encryption"
	pgen "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// SessionRepository persists sessions.
type SessionRepository struct {
	repoBase
}

// NewSessionRepository creates a new session repository.
func NewSessionRepository(db pgen.DBTX, enc encryption.Encryptor) *SessionRepository {
	return &SessionRepository{repoBase{db: db, enc: enc}}
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *SessionRepository) WithTx(tx transaction.Tx) (application.SessionRepository, error) {
	dbtx, err := assertTxDB(tx)
	if err != nil {
		return nil, fmt.Errorf("identity.SessionRepository.WithTx: %w", err)
	}
	return NewSessionRepository(dbtx, r.enc), nil
}

// inetParam converts a stored IP string into the netip.Addr the generated
// INET columns carry. A false valid result means SQL NULL (an absent IP); an
// unparsable non-empty value is a data bug and surfaces as an error.
func inetParam(ip string) (addr netip.Addr, valid bool, err error) {
	if ip == "" {
		return netip.Addr{}, false, nil
	}
	addr, err = netip.ParseAddr(ip)
	if err != nil {
		return netip.Addr{}, false, fmt.Errorf("parse session ip %q: %w", ip, err)
	}
	return addr, true, nil
}

// inetParamPtr is the pointer-shaped variant the generated params carry:
// nil maps to SQL NULL.
func inetParamPtr(ip string) (*netip.Addr, error) {
	addr, valid, err := inetParam(ip)
	if err != nil || !valid {
		return nil, err
	}
	return &addr, nil
}

// cityParam maps a city name onto the nullable text column: an empty value is
// SQL NULL, never an empty string, so an unresolved city cannot erase a
// stored one.
func cityParam(city string) pgtype.Text {
	return pgtype.Text{String: city, Valid: city != ""}
}

// browserMajorParam maps the major onto the nullable int column: 0 (unknown)
// is SQL NULL, values above int32 do not exist in browser versioning and are
// clamped to unknown rather than overflowing.
func browserMajorParam(major int) pgtype.Int4 {
	if major <= 0 || major > math.MaxInt32 {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(major), Valid: true}
}

func (r *SessionRepository) Create(ctx context.Context, session domain.Session) error {
	lastIPParam, err := inetParamPtr(session.LastIP)
	if err != nil {
		return err
	}
	_, err = r.q().CreateSession(ctx, pgen.CreateSessionParams{
		ID:           pgconv.UUIDToPgtype(session.ID),
		UserID:       pgconv.UUIDToPgtype(session.UserID),
		TokenHash:    session.TokenHash,
		ExpiresAt:    pgtype.Timestamptz{Time: session.ExpiresAt, Valid: true},
		LastUsedAt:   pgtype.Timestamptz{Time: session.LastUsedAt, Valid: true},
		RotatedAt:    pgtype.Timestamptz{Time: session.RotatedAt, Valid: true},
		LastIp:       lastIPParam,
		UserAgent:    session.UserAgent,
		DeviceType:   string(session.DeviceType),
		Browser:      session.Browser,
		BrowserMajor: browserMajorParam(session.BrowserMajor),
		Os:           session.OS,
		City:         cityParam(session.City),
	})
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (r *SessionRepository) Touch(ctx context.Context, session domain.Session) error {
	lastIPParam, err := inetParamPtr(session.LastIP)
	if err != nil {
		return err
	}
	if err := r.q().TouchSession(ctx, pgen.TouchSessionParams{
		TokenHash:  session.TokenHash,
		ExpiresAt:  pgtype.Timestamptz{Time: session.ExpiresAt, Valid: true},
		LastUsedAt: pgtype.Timestamptz{Time: session.LastUsedAt, Valid: true},
		LastIp:     lastIPParam,
		City:       cityParam(session.City),
	}); err != nil {
		return fmt.Errorf("touch session: %w", err)
	}
	return nil
}

func (r *SessionRepository) Rotate(ctx context.Context, session domain.Session, newTokenHash string) (bool, error) {
	lastIPParam, err := inetParamPtr(session.LastIP)
	if err != nil {
		return false, err
	}
	n, err := r.q().RotateSessionToken(ctx, pgen.RotateSessionTokenParams{
		ID:           pgconv.UUIDToPgtype(session.ID),
		NewTokenHash: newTokenHash,
		OldTokenHash: session.TokenHash,
		RotatedAt:    pgtype.Timestamptz{Time: session.RotatedAt, Valid: true},
		ExpiresAt:    pgtype.Timestamptz{Time: session.ExpiresAt, Valid: true},
		LastUsedAt:   pgtype.Timestamptz{Time: session.LastUsedAt, Valid: true},
		LastIp:       lastIPParam,
		City:         cityParam(session.City),
	})
	if err != nil {
		return false, fmt.Errorf("rotate session token: %w", err)
	}
	return n == 1, nil
}

func (r *SessionRepository) DeleteByTokenHash(ctx context.Context, tokenHash string) error {
	if err := r.q().DeleteSessionByTokenHash(ctx, tokenHash); err != nil {
		return fmt.Errorf("delete session by token hash: %w", err)
	}
	return nil
}

func (r *SessionRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	if err := r.q().DeleteSessionsByUserID(ctx, pgconv.UUIDToPgtype(userID)); err != nil {
		return fmt.Errorf("delete sessions by user id: %w", err)
	}
	return nil
}

func (r *SessionRepository) DeleteByUserIDExcept(ctx context.Context, userID uuid.UUID, tokenHash string) (int64, error) {
	n, err := r.q().DeleteSessionsByUserIDExcept(ctx, pgen.DeleteSessionsByUserIDExceptParams{
		UserID:    pgconv.UUIDToPgtype(userID),
		TokenHash: tokenHash,
	})
	if err != nil {
		return 0, fmt.Errorf("delete sessions by user id except: %w", err)
	}
	return n, nil
}

func (r *SessionRepository) DeleteByIDForUser(ctx context.Context, id, userID uuid.UUID) (bool, error) {
	n, err := r.q().DeleteSessionByIDForUser(ctx, pgen.DeleteSessionByIDForUserParams{
		ID:     pgconv.UUIDToPgtype(id),
		UserID: pgconv.UUIDToPgtype(userID),
	})
	if err != nil {
		return false, fmt.Errorf("delete session by id: %w", err)
	}
	return n == 1, nil
}

func (r *SessionRepository) DeleteExpiredBefore(ctx context.Context, before time.Time) (int64, error) {
	total, err := deleteBatched(ctx, before, func(ctx context.Context, before time.Time, limit int32) (int64, error) {
		n, err := r.q().DeleteExpiredSessionsBatch(ctx, pgen.DeleteExpiredSessionsBatchParams{
			ExpiresAt: pgtype.Timestamptz{Time: before, Valid: true},
			Limit:     limit,
		})
		if err != nil {
			return 0, fmt.Errorf("delete expired sessions batch: %w", err)
		}
		return n, nil
	})
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	return total, nil
}

func (r *SessionRepository) GetByTokenHash(ctx context.Context, tokenHash string, now time.Time) (domain.Session, domain.User, error) {
	row, err := r.q().GetSessionByTokenHash(ctx, pgen.GetSessionByTokenHashParams{
		TokenHash:   tokenHash,
		GraceCutoff: pgtype.Timestamptz{Time: now.Add(-domain.SessionRotationGrace), Valid: true},
		SeenAfter:   pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		if notFound(err) {
			return domain.Session{}, domain.User{}, application.ErrNotFound
		}
		return domain.Session{}, domain.User{}, fmt.Errorf("get session by token hash: %w", err)
	}

	user, err := mapUser(ctx, r.enc, userSourceFromSession(row).toUserRow())
	if err != nil {
		return domain.Session{}, domain.User{}, err
	}

	session := mapSessionCore(sessionColumns{
		id: row.ID, userID: row.UserID, tokenHash: row.TokenHash,
		expiresAt: row.ExpiresAt, createdAt: row.CreatedAt, lastUsedAt: row.LastUsedAt,
		rotatedAt: row.RotatedAt, previousTokenHash: row.PreviousTokenHash,
		lastIP: row.LastIp, userAgent: row.UserAgent, deviceType: row.DeviceType,
		browser: row.Browser, browserMajor: row.BrowserMajor, os: row.Os, city: row.City,
	})
	return session, user, nil
}

func (r *SessionRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Session, error) {
	rows, err := r.q().ListSessionsByUserID(ctx, pgconv.UUIDToPgtype(userID))
	if err != nil {
		return nil, fmt.Errorf("list sessions by user id: %w", err)
	}
	sessions := make([]domain.Session, 0, len(rows))
	for _, row := range rows {
		sessions = append(sessions, mapSessionCore(sessionColumns{
			id: row.ID, userID: pgconv.UUIDToPgtype(userID), tokenHash: row.TokenHash,
			expiresAt: row.ExpiresAt, createdAt: row.CreatedAt, lastUsedAt: row.LastUsedAt,
			lastIP: row.LastIp, deviceType: row.DeviceType,
			browser: row.Browser, browserMajor: row.BrowserMajor, os: row.Os, city: row.City,
		}))
	}
	return sessions, nil
}

func (r *SessionRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.Session, error) {
	row, err := r.q().GetSessionByID(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		if notFound(err) {
			return domain.Session{}, application.ErrNotFound
		}
		return domain.Session{}, fmt.Errorf("get session by id: %w", err)
	}
	return mapSessionCore(sessionColumns{
		id: row.ID, userID: row.UserID, tokenHash: row.TokenHash,
		expiresAt: row.ExpiresAt, createdAt: row.CreatedAt, lastUsedAt: row.LastUsedAt,
		lastIP: row.LastIp, deviceType: row.DeviceType,
		browser: row.Browser, browserMajor: row.BrowserMajor, os: row.Os, city: row.City,
	}), nil
}

// sessionColumns is the union of the session columns across the sqlc rows that
// map onto a domain.Session. Rotation bookkeeping and the raw User-Agent live
// only on the auth lookup path; the list/read-by-id rows leave those fields
// empty (the devices list never needs them).
type sessionColumns struct {
	id, userID        pgtype.UUID
	tokenHash         string
	expiresAt         pgtype.Timestamptz
	createdAt         pgtype.Timestamptz
	lastUsedAt        pgtype.Timestamptz
	rotatedAt         pgtype.Timestamptz
	previousTokenHash pgtype.Text
	lastIP            *netip.Addr
	userAgent         string
	deviceType        string
	browser           string
	browserMajor      pgtype.Int4
	os                string
	city              pgtype.Text
}

// mapSessionCore builds a domain.Session from a sqlc session row.
func mapSessionCore(c sessionColumns) domain.Session {
	s := domain.Session{
		ID:         pgconv.UUIDFromPgtype(c.id),
		UserID:     pgconv.UUIDFromPgtype(c.userID),
		TokenHash:  c.tokenHash,
		ExpiresAt:  c.expiresAt.Time,
		CreatedAt:  c.createdAt.Time,
		LastUsedAt: c.lastUsedAt.Time,
		RotatedAt:  c.rotatedAt.Time,
		UserAgent:  c.userAgent,
		OS:         c.os,
	}
	if c.previousTokenHash.Valid {
		s.PreviousTokenHash = c.previousTokenHash.String
	}
	if c.lastIP != nil {
		s.LastIP = c.lastIP.String()
	}
	if c.deviceType != "" {
		s.DeviceType = domain.DeviceType(c.deviceType)
	} else {
		s.DeviceType = domain.DeviceUnknown
	}
	s.Browser = c.browser
	if c.browserMajor.Valid {
		s.BrowserMajor = int(c.browserMajor.Int32)
	}
	if c.city.Valid {
		s.City = c.city.String
	}
	return s
}
