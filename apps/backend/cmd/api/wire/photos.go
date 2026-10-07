package wire

import (
	"context"
	"fmt"

	"github.com/nambers/arenda-planform/apps/backend/internal/platform/objectstorage"
	storageshared "github.com/nambers/arenda-planform/apps/backend/internal/shared/storage"
)

// WirePhotoStorage constructs the one private-photos object storage of the
// process (ADR 0065): the S3 adapter against Рег.ру when PHOTO_STORAGE_PROVIDER=s3,
// the in-memory fake otherwise (local dev and tests, ADR 0005 — no MinIO).
// The S3 bucket is health-checked at startup: a misconfigured bucket must
// fail the boot loudly, not the first photo request.
func WirePhotoStorage(ctx context.Context, p platformDeps) (storageshared.PhotoStorage, error) {
	if p.Cfg.PhotoStorageS3Enabled {
		s3Storage, err := objectstorage.NewS3Storage(
			ctx,
			p.Cfg.PhotoStorageEndpoint,
			p.Cfg.PhotoStorageRegion,
			p.Cfg.PhotoStorageBucket,
			p.Cfg.PhotoStorageAccessKey,
			p.Cfg.PhotoStorageSecretKey,
		)
		if err != nil {
			return nil, fmt.Errorf("photo storage: %w", err)
		}
		if err := s3Storage.HeadBucket(ctx); err != nil {
			return nil, fmt.Errorf("photo storage: head bucket %q: %w", p.Cfg.PhotoStorageBucket, err)
		}
		p.Logger.InfoContext(ctx, "photo storage initialized",
			"provider", "s3", "bucket", p.Cfg.PhotoStorageBucket, "endpoint", p.Cfg.PhotoStorageEndpoint)
		return s3Storage, nil
	}

	p.Logger.InfoContext(ctx, "photo storage initialized", "provider", "fake")
	return objectstorage.NewFakeStorage(), nil
}
