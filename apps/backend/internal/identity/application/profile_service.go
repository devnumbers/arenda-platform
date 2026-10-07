package application

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/photo"
	storageshared "github.com/nambers/arenda-planform/apps/backend/internal/shared/storage"
)

// ProfileService provides read and update operations for the user's own profile.
//
// It embeds the identity txStoreFactory so UpdateProfile runs through runInTx
// (ADR 0033, migration step 3): the user repository and audit recorder are
// bound to the transaction by the factory, and the use case describes only
// the business logic. Me is a plain read and stays outside the transaction.
type ProfileService struct {
	txStoreFactory
	// The photos port streams the profile photo (ADR 0065): the upload lands in the
	// private storage under a fresh UUID key, the serving endpoints open it
	// back — no public URL ever exists.
	photos storageshared.PhotoStorage
	// The logger reports the best-effort orphan cleanups.
	logger *slog.Logger
}

// NewProfileService creates a ProfileService. It embeds the shared identity
// txStoreFactory so UpdateProfile runs through runInTx; the repositories and
// audit recorder are shared by every identity service (ADR 0033 γ-factory).
// The photos port streams the profile photo (ADR 0065); the logger is the
// orphan-cleanup channel.
func NewProfileService(factory txStoreFactory, photos storageshared.PhotoStorage, logger *slog.Logger) *ProfileService {
	return &ProfileService{txStoreFactory: factory, photos: photos, logger: logger}
}

// Me returns the user profile.
func (s *ProfileService) Me(ctx context.Context, userID uuid.UUID) (domain.User, error) {
	return s.users.GetByID(ctx, userID)
}

// UpdateProfile updates the user's personal data. The email is not part of
// the profile contract: it changes only through the confirmed two-code flow
// (issue #721). The mutation and the audit entry run in a single transaction
// through runInTx (ADR 0033, ADR 0020).
func (s *ProfileService) UpdateProfile(ctx context.Context, userID uuid.UUID, cmd UpdateProfileCommand) (domain.User, error) {
	var updated domain.User

	err := s.runInTx(ctx, func(stores *txStores) error {
		user, err := stores.users.GetByIDForUpdate(ctx, userID)
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}

		if err := user.UpdatePersonalData(cmd.Name, cmd.Surname, cmd.Patronymic, cmd.Timezone); err != nil {
			return fmt.Errorf("update personal data: %w", err)
		}

		updated, err = stores.users.Update(ctx, user)
		if err != nil {
			return fmt.Errorf("update user: %w", err)
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &userID,
			ActorRole:  auditdomain.ActorRoleFromRole(updated.Role),
			Action:     auditdomain.ActionProfileUpdated,
			EntityType: auditdomain.EntityUser,
			EntityID:   &userID,
			Context:    map[string]any{"fields": cmd.ChangedFields()},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.User{}, err
	}

	return updated, nil
}

// PhotoDescriptor resolves the profile photo's identity for the serving
// endpoint (ADR 0065): the key powers the ETag — a replacement mints a new
// UUID key, so a 304 never lies about the bytes — and the content type
// labels the response. The profile photo is readable by the owner only: the
// session-scoped userID is the whole access check.
func (s *ProfileService) PhotoDescriptor(ctx context.Context, userID uuid.UUID) (key, contentType string, err error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return "", "", err
	}
	if user.PhotoKey == nil {
		return "", "", ErrPhotoNotFound
	}
	return *user.PhotoKey, derefString(user.PhotoContentType), nil
}

// OpenPhoto opens the profile photo object (ADR 0065): the body, its byte
// size, the stored content type and the key (the ETag source). A column
// key without an object is an orphan — the honest ErrPhotoNotFound.
func (s *ProfileService) OpenPhoto(
	ctx context.Context, userID uuid.UUID,
) (body io.ReadCloser, size int64, contentType, key string, err error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, 0, "", "", err
	}
	if user.PhotoKey == nil {
		return nil, 0, "", "", ErrPhotoNotFound
	}
	body, size, contentType, err = s.photos.Open(ctx, *user.PhotoKey)
	if err != nil {
		if errors.Is(err, storageshared.ErrNotFound) {
			return nil, 0, "", "", ErrPhotoNotFound
		}
		return nil, 0, "", "", fmt.Errorf("open profile photo: %w", err)
	}
	return body, size, contentType, *user.PhotoKey, nil
}

// SetProfilePhoto replaces the profile photo (ADR 0065, one image per
// entity): the processed upload lands under a fresh UUID key, the columns
// flip in the same transaction, and the previous object is removed
// best-effort after the commit — a failed cleanup leaves a logged orphan,
// never a broken state.
func (s *ProfileService) SetProfilePhoto(ctx context.Context, userID uuid.UUID, processed photo.Processed) (domain.User, error) {
	key, err := photo.NewKey(processed.ContentType)
	if err != nil {
		return domain.User{}, err
	}

	var (
		updated domain.User
		oldKey  *string
	)
	err = s.runInTx(ctx, func(stores *txStores) error {
		user, err := stores.users.GetByIDForUpdate(ctx, userID)
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}
		oldKey = user.PhotoKey

		if err := s.photos.Put(ctx, key, bytes.NewReader(processed.Data), processed.ContentType, processed.Size); err != nil {
			return fmt.Errorf("put profile photo: %w", err)
		}

		updated, err = stores.users.SetPhoto(ctx, userID, &key, &processed.ContentType)
		if err != nil {
			return fmt.Errorf("set user photo: %w", err)
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &userID,
			ActorRole:  auditdomain.ActorRoleFromRole(updated.Role),
			Action:     auditdomain.ActionProfilePhotoAdded,
			EntityType: auditdomain.EntityProfilePhoto,
			EntityID:   &userID,
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
	if err != nil {
		// The new object may have landed before a later step failed: it is
		// unreferenced — remove it the same best-effort way as the replaced
		// one.
		s.cleanupPhoto(ctx, key)
		return domain.User{}, err
	}
	if oldKey != nil {
		s.cleanupPhoto(ctx, *oldKey)
	}
	return updated, nil
}

// DeleteProfilePhoto clears the photo columns; clearing an absent photo is
// ErrPhotoNotFound — the transport's 404. The object removal stays
// best-effort after the commit.
func (s *ProfileService) DeleteProfilePhoto(ctx context.Context, userID uuid.UUID) (domain.User, error) {
	var (
		updated domain.User
		oldKey  *string
	)
	err := s.runInTx(ctx, func(stores *txStores) error {
		user, err := stores.users.GetByIDForUpdate(ctx, userID)
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}
		if user.PhotoKey == nil {
			return ErrPhotoNotFound
		}
		oldKey = user.PhotoKey

		updated, err = stores.users.SetPhoto(ctx, userID, nil, nil)
		if err != nil {
			return fmt.Errorf("set user photo: %w", err)
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &userID,
			ActorRole:  auditdomain.ActorRoleFromRole(updated.Role),
			Action:     auditdomain.ActionProfilePhotoDeleted,
			EntityType: auditdomain.EntityProfilePhoto,
			EntityID:   &userID,
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.User{}, err
	}
	if oldKey != nil {
		s.cleanupPhoto(ctx, *oldKey)
	}
	return updated, nil
}

// cleanupPhoto removes an object best-effort: a failure is logged as an
// orphan — it never fails the mutation that already committed.
func (s *ProfileService) cleanupPhoto(ctx context.Context, key string) {
	if err := s.photos.Delete(ctx, key); err != nil {
		s.logger.WarnContext(ctx, "failed to delete profile photo from storage",
			slog.String("key", key),
			slog.String("error", err.Error()),
		)
	}
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
