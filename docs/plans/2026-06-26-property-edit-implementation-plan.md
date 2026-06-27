# Property Edit Page Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the property edit page at `/properties/[id]/edit` and the backend `DELETE /properties/{propertyId}/photos/{photoId}` endpoint, following the approved design in `docs/plans/2026-06-26-property-edit-design.md`.

**Architecture:** A single-screen client form widget reuses existing project UI primitives (`TextField`, `Button`, `IconLink`) and data hooks (`useProperty`, `useUpdateProperty`, `useUploadPropertyPhoto`). A new custom `PropertyTypeSelect` mimics the Figma dropdown with Hero UI `Popover` + `Listbox`. The backend photo delete endpoint extends the existing storage and photo repository ports.

**Tech Stack:** Next.js 16 + React 19 + TypeScript + Hero UI v3 + TanStack Query; Go backend with sqlc, oapi-codegen, S3-compatible storage.

---

## Notes before starting

- Tests are out of scope for this task. Do not write or modify test files.
- Work in the current checkout (no worktree) per repository rules.
- Required skills to invoke during implementation: `$go`, `$next-best-practices`, `$vercel-react-best-practices`.
- Run generation commands from `apps/backend` after OpenAPI/SQL changes.
- Frontend API types are generated from `apps/frontend` via `npm run generate:api`.

---

## Task 1: Add DELETE photo endpoint to OpenAPI spec

**Files:**
- Modify: `apps/backend/api/openapi/openapi.yaml:309-346`

**Step 1: Add the delete operation under `/properties/{propertyId}/photos/{photoId}`**

Insert after the existing `post` block for `/properties/{propertyId}/photos`:

```yaml
    delete:
      operationId: deletePropertyPhoto
      security:
        - sessionCookie: []
      parameters:
        - name: propertyId
          in: path
          required: true
          schema:
            type: string
            format: uuid
        - name: photoId
          in: path
          required: true
          schema:
            type: string
            format: uuid
      responses:
        '204':
          description: Photo deleted
        '400':
          $ref: '#/components/responses/BadRequest'
        '401':
          $ref: '#/components/responses/Unauthorized'
        '404':
          $ref: '#/components/responses/NotFound'
```

**Step 2: Run oapi-codegen**

```bash
cd apps/backend
go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.0 -config api/openapi/oapi-codegen.yaml api/openapi/openapi.yaml
```

Expected: `internal/platform/openapi/generated.gen.go` is regenerated and contains `DeletePropertyPhoto` in the server interface.

---

## Task 2: Add photo delete/get sqlc queries

**Files:**
- Modify: `apps/backend/db/queries/property_photos.sql`

**Step 1: Append two queries**

```sql
-- name: GetPropertyPhotoByID :one
SELECT * FROM property_photos WHERE id = $1;

-- name: DeletePropertyPhoto :exec
DELETE FROM property_photos WHERE id = $1;
```

**Step 2: Regenerate sqlc**

```bash
cd apps/backend
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
```

Expected: `internal/platform/generated/postgres/property_photos.sql.go` now contains `GetPropertyPhotoByID` and `DeletePropertyPhoto`.

---

## Task 3: Extend PhotoStorage port and adapters

**Files:**
- Modify: `apps/backend/internal/properties/application/ports.go:63-66`
- Modify: `apps/backend/internal/properties/adapters/storage/s3.go`
- Modify: `apps/backend/internal/properties/adapters/storage/fake.go`

**Step 1: Add Delete to the port interface**

```go
// PhotoStorage persists uploaded property photos and returns their public URL.
type PhotoStorage interface {
	Upload(ctx context.Context, key string, contentType string, data io.Reader) (string, error)
	Delete(ctx context.Context, key string) error
}
```

**Step 2: Implement Delete in S3Storage**

Add to `apps/backend/internal/properties/adapters/storage/s3.go`:

```go
// Delete removes the object with the given key from S3.
func (s *S3Storage) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete object: %w", err)
	}
	return nil
}
```

**Step 3: Implement Delete in FakeStorage**

Add to `apps/backend/internal/properties/adapters/storage/fake.go`:

```go
// Delete removes the object from the in-memory store.
func (s *FakeStorage) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, key)
	return nil
}
```

---

## Task 4: Extend PropertyPhotoRepository port and adapter

**Files:**
- Modify: `apps/backend/internal/properties/application/ports.go:68-75`
- Modify: `apps/backend/internal/properties/adapters/postgres/property_photo_repository.go`

**Step 1: Extend the repository interface**

```go
// PropertyPhotoRepository persists photo metadata for properties.
type PropertyPhotoRepository interface {
	Create(ctx context.Context, propertyID uuid.UUID, url string) (domain.Photo, error)
	GetByID(ctx context.Context, photoID uuid.UUID) (domain.Photo, error)
	GetByPropertyID(ctx context.Context, propertyID uuid.UUID) ([]domain.Photo, error)
	GetByPropertyIDs(ctx context.Context, propertyIDs []uuid.UUID) (map[uuid.UUID][]domain.Photo, error)
	CountByPropertyID(ctx context.Context, propertyID uuid.UUID) (int, error)
	Delete(ctx context.Context, photoID uuid.UUID) error
	WithTx(tx transaction.Tx) PropertyPhotoRepository
}
```

**Step 2: Implement GetByID and Delete in postgres adapter**

Add to `apps/backend/internal/properties/adapters/postgres/property_photo_repository.go` after `CountByPropertyID`:

```go
// GetByID returns a single photo by its ID.
func (r *PropertyPhotoRepository) GetByID(ctx context.Context, photoID uuid.UUID) (domain.Photo, error) {
	row, err := r.q().GetPropertyPhotoByID(ctx, pgconv.UUIDToPgtype(photoID))
	if err != nil {
		return domain.Photo{}, err
	}
	return photoFromRow(row), nil
}

// Delete removes a photo record by its ID.
func (r *PropertyPhotoRepository) Delete(ctx context.Context, photoID uuid.UUID) error {
	return r.q().DeletePropertyPhoto(ctx, pgconv.UUIDToPgtype(photoID))
}
```

---

## Task 5: Add DeletePropertyPhoto service method

**Files:**
- Modify: `apps/backend/internal/properties/application/service.go`

**Step 1: Add DeletePropertyPhoto after AddPropertyPhoto**

```go
// DeletePropertyPhoto removes a photo from storage and the database.
func (s *PropertyService) DeletePropertyPhoto(ctx context.Context, ownerID, propertyID, photoID uuid.UUID) error {
	_, err := s.repo.GetByIDAndOwner(ctx, propertyID, ownerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("get property: %w", err)
	}

	photo, err := s.photoRepo.GetByID(ctx, photoID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("get photo: %w", err)
	}

	key := strings.TrimPrefix(photo.URL, s.photoStoragePublicBaseURL+"/")
	if err := s.photoStorage.Delete(ctx, key); err != nil {
		return fmt.Errorf("delete photo from storage: %w", err)
	}

	if err := s.photoRepo.Delete(ctx, photoID); err != nil {
		return fmt.Errorf("delete photo record: %w", err)
	}

	return nil
}
```

Wait — the service does not currently store `photoStoragePublicBaseURL`. Two options:

**Option A (recommended):** derive the storage key from the photo ID and property ID using the known prefix pattern, instead of parsing the URL.

```go
key := fmt.Sprintf("%s/%s/%s%s", photoKeyPrefix, propertyID.String(), photoID.String(), path.Ext(photo.URL))
```

This requires importing `path` and assumes the extension in the URL matches the stored extension. Use this only if photo upload never changes extensions.

**Option B:** add `photoStoragePublicBaseURL string` to `PropertyService` and pass it via `NewPropertyService`.

Choose **Option A** to minimize wiring changes. If the URL structure is unreliable, switch to Option B.

With Option A, the method becomes:

```go
import "path"

// DeletePropertyPhoto removes a photo from storage and the database.
func (s *PropertyService) DeletePropertyPhoto(ctx context.Context, ownerID, propertyID, photoID uuid.UUID) error {
	_, err := s.repo.GetByIDAndOwner(ctx, propertyID, ownerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("get property: %w", err)
	}

	photo, err := s.photoRepo.GetByID(ctx, photoID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("get photo: %w", err)
	}

	ext := path.Ext(photo.URL)
	key := fmt.Sprintf("%s/%s/%s%s", photoKeyPrefix, propertyID.String(), photoID.String(), ext)

	if err := s.photoStorage.Delete(ctx, key); err != nil {
		return fmt.Errorf("delete photo from storage: %w", err)
	}

	if err := s.photoRepo.Delete(ctx, photoID); err != nil {
		return fmt.Errorf("delete photo record: %w", err)
	}

	return nil
}
```

Also add `database/sql` import for `sql.ErrNoRows`.

---

## Task 6: Add DeletePropertyPhoto HTTP handler

**Files:**
- Modify: `apps/backend/internal/platform/httpapi/property_handlers.go`

**Step 1: Add the handler method**

Insert after `UploadPropertyPhoto`:

```go
// DeletePropertyPhoto implements DELETE /properties/{propertyId}/photos/{photoId}.
func (h *PropertyHandlers) DeletePropertyPhoto(w http.ResponseWriter, r *http.Request, propertyId, photoId uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	if err := h.svc.DeletePropertyPhoto(r.Context(), ownerID, propertyId, photoId); err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

**Step 2: Build the backend**

```bash
cd apps/backend && go build ./cmd/api
```

Expected: build succeeds.

---

## Task 7: Add useDeletePropertyPhoto frontend hook

**Files:**
- Modify: `apps/frontend/features/properties/api/hooks.ts`
- Modify: `apps/frontend/features/properties/api/index.ts`

**Step 1: Add the hook after useUploadPropertyPhoto**

```ts
export function useDeletePropertyPhoto(): UseMutationResult<
  void,
  ApiError,
  { propertyId: string; photoId: string }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ propertyId, photoId }) =>
      apiClient<void>(`/properties/${propertyId}/photos/${photoId}`, {
        method: 'DELETE',
      }),
    onSuccess: (_, { propertyId }) => {
      queryClient.invalidateQueries({ queryKey: propertyKeys.list });
      queryClient.invalidateQueries({ queryKey: propertyKeys.detail(propertyId) });
    },
  });
}
```

**Step 2: Export it from the barrel file**

In `apps/frontend/features/properties/api/index.ts` add `useDeletePropertyPhoto` to the re-export list.

**Step 3: Regenerate frontend API types**

```bash
cd apps/frontend && npm run generate:api
```

Expected: `shared/api/generated.ts` is updated and contains the new DELETE endpoint types.

---

## Task 8: Create PropertyTypeSelect component

**Files:**
- Create: `apps/frontend/widgets/properties/ui/PropertyTypeSelect.tsx`
- Create: `apps/frontend/widgets/properties/ui/PropertyTypeSelect.module.css`

**Step 1: Implement the select**

```tsx
'use client';

import { useState, type JSX } from 'react';
import {
  Popover,
  PopoverTrigger,
  PopoverContent,
  Listbox,
  ListboxItem,
} from '@heroui/react';
import { TextField } from '@/shared/ui/text-field';
import { IconButton } from '@/shared/ui/icon-button';
import { ChevronDown } from '@/shared/assets/icons';
import { propertyTypeOptions } from '@/features/properties/lib/property-types';
import type { PropertyType } from '@/entities/property/model/types';
import styles from './PropertyTypeSelect.module.css';

export type PropertyTypeSelectProps = {
  value?: PropertyType;
  onChange: (value: PropertyType) => void;
};

export function PropertyTypeSelect({ value, onChange }: PropertyTypeSelectProps): JSX.Element {
  const [isOpen, setIsOpen] = useState(false);
  const selectedLabel = propertyTypeOptions.find((o) => o.value === value)?.label ?? '';

  return (
    <Popover isOpen={isOpen} onOpenChange={setIsOpen} placement="bottom-start">
      <PopoverTrigger>
        <div className={styles.trigger}>
          <TextField
            label="Тип"
            value={selectedLabel}
            readOnly
            fullWidth
            icon={
              <IconButton
                type="button"
                variant="icon-black"
                size="tiny"
                icon={<ChevronDown />}
                aria-label="Выбрать тип"
              />
            }
          />
        </div>
      </PopoverTrigger>
      <PopoverContent className={styles.dropdown}>
        <Listbox
          aria-label="Тип объекта"
          selectedKeys={value ? new Set([value]) : new Set()}
          onSelectionChange={(keys) => {
            const selected = Array.from(keys as Set<PropertyType>)[0];
            if (selected) {
              onChange(selected);
              setIsOpen(false);
            }
          }}
        >
          {propertyTypeOptions.map((option) => (
            <ListboxItem key={option.value} textValue={option.label}>
              {option.label}
            </ListboxItem>
          ))}
        </Listbox>
      </PopoverContent>
    </Popover>
  );
}
```

**Step 2: Basic styles**

```css
.trigger {
  cursor: pointer;
}

.dropdown {
  width: 100%;
  max-height: 320px;
  overflow-y: auto;
}
```

---

## Task 9: Create PropertyEditForm widget

**Files:**
- Create: `apps/frontend/widgets/properties/ui/PropertyEditForm.tsx`
- Create: `apps/frontend/widgets/properties/ui/PropertyEditForm.module.css`

**Step 1: Implement the form widget**

```tsx
'use client';

import { useCallback, useEffect, useMemo, useState, type JSX } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { toast } from 'react-toastify';
import { ROUTES } from '@/shared/config/routes';
import {
  useProperty,
  useUpdateProperty,
  useUploadPropertyPhoto,
  useDeletePropertyPhoto,
} from '@/features/properties/api';
import { PropertyDetailError } from '@/widgets/property-detail';
import { PropertyTypeSelect } from './PropertyTypeSelect';
import { Button } from '@/shared/ui/button';
import { TextField } from '@/shared/ui/text-field';
import { IconLink } from '@/shared/ui/icon-link';
import { ArrowLeft } from '@/shared/assets/icons';
import { AddressField } from './AddressField';
import { PhotoGrid } from './PhotoGrid';
import styles from './PropertyEditForm.module.css';

const MAX_NAME_LENGTH = 50;
const MAX_DESCRIPTION_LENGTH = 500;

export function PropertyEditForm(): JSX.Element {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const propertyQuery = useProperty(id ?? '');
  const updateProperty = useUpdateProperty();
  const uploadPhoto = useUploadPropertyPhoto();
  const deletePhoto = useDeletePropertyPhoto();

  const [type, setType] = useState<PropertyType | undefined>();
  const [address, setAddress] = useState('');
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [newPhotos, setNewPhotos] = useState<File[]>([]);
  const [deletedPhotoIds, setDeletedPhotoIds] = useState<string[]>([]);

  useEffect(() => {
    if (propertyQuery.data) {
      setType(propertyQuery.data.type);
      setAddress(propertyQuery.data.address);
      setName(propertyQuery.data.name);
      setDescription(propertyQuery.data.description ?? '');
    }
  }, [propertyQuery.data]);

  const isNameValid = name.trim().length > 0;
  const isAddressValid = address.trim().length > 0;
  const isTypeValid = Boolean(type);
  const canSubmit = isNameValid && isAddressValid && isTypeValid && !updateProperty.isPending;

  const existingPhotos = useMemo(() => propertyQuery.data?.photos ?? [], [propertyQuery.data]);

  const handleSubmit = useCallback(async () => {
    if (!id || !type || !address.trim() || !name.trim()) return;

    try {
      await updateProperty.mutateAsync({
        id,
        data: { name: name.trim(), type, address: address.trim(), description: description.trim() || undefined },
      });

      await Promise.all(deletedPhotoIds.map((photoId) => deletePhoto.mutateAsync({ propertyId: id, photoId })));
      await Promise.all(newPhotos.map((file) => uploadPhoto.mutateAsync({ propertyId: id, file })));

      toast.success('Объект обновлён');
      router.push(ROUTES.property(id));
    } catch (error) {
      toast.error('Не удалось сохранить изменения');
    }
  }, [id, type, address, name, description, deletedPhotoIds, newPhotos, updateProperty, deletePhoto, uploadPhoto, router]);

  if (propertyQuery.isError) {
    return <PropertyDetailError onRetry={() => propertyQuery.refetch()} isLoading={propertyQuery.isFetching} />;
  }

  if (propertyQuery.isPending) {
    return <div className={styles.loading}>Загрузка...</div>;
  }

  return (
    <div className={styles.root}>
      <header className={styles.header}>
        <IconLink href={ROUTES.property(id ?? '')} aria-label="Назад" icon={<ArrowLeft />} />
        <h1 className={styles.title}>Информация об объекте</h1>
        <span className={styles.placeholder} aria-hidden="true" />
      </header>

      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>Данные</h2>
        <div className={styles.fields}>
          <PropertyTypeSelect value={type} onChange={setType} />
          <AddressField value={address} onChange={setAddress} />
          <TextField
            label="Название"
            required
            fullWidth
            maxLength={MAX_NAME_LENGTH}
            value={name}
            onChange={(e) => setName(e.currentTarget.value)}
          />
          <TextField
            label="Описание"
            multiline
            fullWidth
            maxLength={MAX_DESCRIPTION_LENGTH}
            value={description}
            onChange={(e) => setDescription(e.currentTarget.value)}
          />
        </div>
      </section>

      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>Фотографии</h2>
        <PhotoGrid
          existingPhotos={existingPhotos}
          newPhotos={newPhotos}
          onNewPhotosChange={setNewPhotos}
          onDeleteExisting={(photoId) => setDeletedPhotoIds((prev) => [...prev, photoId])}
          isUploading={uploadPhoto.isPending}
        />
      </section>

      <Button
        type="button"
        variant="primary"
        size="large"
        fullWidth
        loading={updateProperty.isPending}
        disabled={!canSubmit}
        onClick={handleSubmit}
      >
        Сохранить изменения
      </Button>
    </div>
  );
}
```

**Note:** This plan introduces two helper components, `AddressField` and `PhotoGrid`. You may inline them in the same file or extract them. Extraction is recommended for readability. See Task 10 and Task 11.

**Step 2: Styles**

```css
.root {
  display: flex;
  flex-direction: column;
  gap: 48px;
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.title {
  margin: 0;
  font-size: 14px;
  font-weight: 500;
  line-height: 18px;
  color: var(--color-text);
}

.placeholder {
  width: 40px;
  height: 40px;
  flex-shrink: 0;
  visibility: hidden;
}

.section {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.sectionTitle {
  margin: 0;
  font-size: 20px;
  font-weight: 500;
  line-height: 22px;
  color: var(--color-text);
}

.fields {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.loading {
  padding: 48px 0;
  text-align: center;
  color: var(--color-text-muted);
}
```

---

## Task 10: Create AddressField helper

**Files:**
- Create: `apps/frontend/widgets/properties/ui/AddressField.tsx`
- Create: `apps/frontend/widgets/properties/ui/AddressField.module.css` (copy/adapt from `PropertyAddressStep.module.css`)

**Step 1: Extract a reusable address field**

Copy the address input + dropdown logic from `PropertyAddressStep.tsx` into a new component without the wizard-specific `onNext`/`onBack` buttons.

Key props:

```ts
export type AddressFieldProps = {
  value?: string;
  onChange: (address: string) => void;
};
```

Reuse `useAddressSuggestions`, `useDebounce`, keyboard navigation, and click-outside behavior from `PropertyAddressStep`.

---

## Task 11: Create PhotoGrid helper

**Files:**
- Create: `apps/frontend/widgets/properties/ui/PhotoGrid.tsx`
- Create: `apps/frontend/widgets/properties/ui/PhotoGrid.module.css`

**Step 1: Implement photo grid with add/delete**

Props:

```ts
export type PhotoGridProps = {
  existingPhotos: PropertyPhoto[];
  newPhotos: File[];
  onNewPhotosChange: (photos: File[]) => void;
  onDeleteExisting: (photoId: string) => void;
  isUploading?: boolean;
};
```

Behavior:

- Render existing photos first, then previews of `newPhotos`.
- Each item shows a delete button (trash icon) in the top-right corner.
- Clicking "Добавить фото" opens a hidden `<input type="file" accept="image/jpeg,image/png,image/webp" multiple>`.
- Validate: max 10 total, max 5 MB each, supported formats.
- Show error message below the grid when validation fails.
- When total reaches 10, render a disabled button "Загружено максимум" with subtitle "10 из 10 фото".

Reuse constants and preview logic from `PropertyInfoStep.tsx`.

---

## Task 12: Create the edit page

**Files:**
- Create: `apps/frontend/app/(cabinet)/properties/[id]/edit/page.tsx`
- Create: `apps/frontend/app/(cabinet)/properties/[id]/edit/page.module.css` (copy from `new/page.module.css`)

**Step 1: Page shell**

```tsx
import type { Metadata } from 'next';
import { PropertyEditForm } from '@/widgets/properties';
import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'Редактировать объект — Arenda Platform',
  description: 'Изменение информации об объекте недвижимости',
};

export default async function PropertyEditPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return (
    <div className={styles.root}>
      <div className={styles.content}>
        <PropertyEditForm />
      </div>
    </div>
  );
}
```

---

## Task 13: Export new widgets

**Files:**
- Modify: `apps/frontend/widgets/properties/index.ts`

**Step 1: Add exports**

```ts
export { PropertyEditForm } from './ui/PropertyEditForm';
```

If `PropertyTypeSelect`, `AddressField`, or `PhotoGrid` are intended for reuse, consider exporting them as well. Otherwise keep them private to the widget folder.

---

## Task 14: Verification

**Backend:**

```bash
cd apps/backend && go build ./cmd/api
make backend-lint
```

**Frontend:**

```bash
cd apps/frontend && npm run lint
```

**Manual check:**

1. Start backend (`make backend-run`) and frontend (`cd apps/frontend && npm run dev`).
2. Navigate to a property detail page.
3. Click «Редактировать объект» in the action menu.
4. Change name, type, address, description.
5. Add a new photo.
6. Delete an existing photo.
7. Click «Сохранить изменения».
8. Verify redirect to detail page and updated data.

---

## Execution handoff

**Plan complete and saved to `docs/plans/2026-06-26-property-edit-implementation-plan.md`.**

Two execution options:

1. **Subagent-Driven (this session)** — dispatch fresh subagent per task, review between tasks, fast iteration. Use `@superpowers:subagent-driven-development`.
2. **Parallel Session (separate)** — open a new session with `@superpowers:executing-plans` and batch execution with checkpoints.

Which approach do you prefer?
