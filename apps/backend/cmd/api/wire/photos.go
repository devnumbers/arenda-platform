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

	// The fake keeps photos in memory only: in a strict env (stage,
	// production) that means every upload dies with the process while the
	// DB keys keep pointing into the bucket — warn loudly, the drift must
	// not survive unnoticed (прод-инцидент 20.10: ENV_FILE оставил fake).
	if p.Cfg.AppEnv != "local" && p.Cfg.AppEnv != "dev" {
		p.Logger.WarnContext(ctx, "photo storage is the in-memory fake: photos are lost on restart",
			"provider", "fake", "env", p.Cfg.AppEnv)
	}
	p.Logger.InfoContext(ctx, "photo storage initialized", "provider", "fake")
	return objectstorage.NewFakeStorage(), nil
}
