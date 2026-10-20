package wire

import (
	"context"
	"fmt"

	"github.com/nambers/arenda-planform/apps/backend/internal/platform/config"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/objectstorage"
	storageshared "github.com/nambers/arenda-planform/apps/backend/internal/shared/storage"
)

// WirePhotoStorage constructs the one private-photos object storage of the
// process (ADR 0065): the S3 adapter against Рег.ру when
// PHOTO_STORAGE_PROVIDER=s3, the disk-backed file adapter for local
// development restarts, or the in-memory fake (tests, temporary launches —
// ADR 0005, no MinIO). The S3 bucket and the file root are health-checked
// at startup: a misconfigured storage must fail the boot loudly, not the
// first photo request.
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

	// The disk-backed provider of the local development (ADR 0065): objects
	// survive restarts. The root is health-checked at startup like the S3
	// bucket: a misconfigured storage must fail the boot loudly.
	if p.Cfg.PhotoStorageProvider == config.ProviderFile {
		fileStorage := objectstorage.NewFileStorage(p.Cfg.PhotoStorageFileDir)
		if err := fileStorage.HeadBucket(ctx); err != nil {
			return nil, fmt.Errorf("photo storage: file root %q: %w", p.Cfg.PhotoStorageFileDir, err)
		}
		p.Logger.InfoContext(ctx, "photo storage initialized",
			"provider", "file", "dir", p.Cfg.PhotoStorageFileDir)
		return fileStorage, nil
	}

	// The fake keeps photos in memory only — and so does a container's
	// ephemeral disk: in a strict env (stage, production) any non-s3
	// provider means every upload dies with the process while the DB keys
	// keep pointing into the storage — warn loudly, the drift must not
	// survive unnoticed (прод-инцидент 20.10: ENV_FILE оставил fake).
	if p.Cfg.AppEnv != "local" && p.Cfg.AppEnv != "dev" {
		p.Logger.WarnContext(ctx, "photo storage keeps objects in the process only: photos are lost on restart",
			"provider", p.Cfg.PhotoStorageProvider, "env", p.Cfg.AppEnv)
	}
	p.Logger.InfoContext(ctx, "photo storage initialized", "provider", p.Cfg.PhotoStorageProvider)
	return objectstorage.NewFakeStorage(), nil
}
