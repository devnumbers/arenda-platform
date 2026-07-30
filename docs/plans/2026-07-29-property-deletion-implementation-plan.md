# Property Deletion Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a backend endpoint `DELETE /properties/{id}?mode=cascade|detach` that deletes a property. `cascade` deletes all related data; `detach` keeps leases/operations/recurring-operations/reminders but sets `property_id = NULL`. Open leases block deletion.

**Architecture:** After the migration in Task 1, FKs on `leases`, `operations`, `recurring_operations`, and `reminders` use `ON DELETE SET NULL`. `property_photos` stays `ON DELETE CASCADE`. For `detach` mode, deleting the property automatically detaches child records. For `cascade` mode, the service explicitly deletes child records in the correct order before deleting the property. Ownership and open-lease checks happen in the application service.

**Tech Stack:** Go 1.26, Chi, sqlc 1.31.1, oapi-codegen 2.7.1, PostgreSQL 18, Bruno.

---

### Task 1: Create DB migration to make `property_id` nullable and switch FKs to SET NULL

**Status:** ✅ Done (commit `f81c8f3`).

Files:
- `apps/backend/db/migrations/000086_property_deletion_detach.up.sql`
- `apps/backend/db/migrations/000086_property_deletion_detach.down.sql`

---

### Task 2: Add SQLC queries for cascade delete and property delete

**Files:**
- Modify: `apps/backend/db/queries/properties.sql`
- Modify: `apps/backend/db/queries/leases.sql`
- Modify: `apps/backend/db/queries/operations.sql`

**Step 1: Add property delete query**

Append to `apps/backend/db/queries/properties.sql`:

```sql
-- name: DeleteProperty :exec
DELETE FROM properties
WHERE id = $1 AND owner_id = $2;
```

**Step 2: Add cascade delete queries for leases and operations**

Append to `apps/backend/db/queries/leases.sql`:

```sql
-- name: DeleteLeasesByProperty :exec
DELETE FROM leases
WHERE owner_id = $1 AND property_id = $2;

-- name: DeleteRecurringOperationsByProperty :exec
DELETE FROM recurring_operations
WHERE owner_id = $1 AND property_id = $2;
```

Append to `apps/backend/db/queries/operations.sql` (create the file if it does not exist, otherwise append):

```sql
-- name: DeleteOperationsByProperty :exec
DELETE FROM operations
WHERE owner_id = $1 AND property_id = $2;
```

**Step 3: Run sqlc generate**

Run:
```bash
cd apps/backend
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
```
Expected: `internal/platform/generated/postgres/*.go` updates with new query methods, no errors.

**Step 4: Commit**

```bash
git add apps/backend/db/queries/properties.sql \
        apps/backend/db/queries/leases.sql \
        apps/backend/db/queries/operations.sql \
        apps/backend/internal/platform/generated/postgres
git commit -m "feat(db): add sqlc queries for property cascade delete"
```

---

### Task 3: Add `DeletePropertyMode` domain enum

**Files:**
- Modify: `apps/backend/internal/properties/domain/property.go`

**Step 1: Add enum type and parser after `PropertyStatus`**

```go
// DeletePropertyMode selects how a property is deleted.
type DeletePropertyMode string

const (
	DeletePropertyModeCascade DeletePropertyMode = "cascade"
	DeletePropertyModeDetach  DeletePropertyMode = "detach"
)

var ErrInvalidDeletePropertyMode = errors.New("invalid delete property mode")

func ParseDeletePropertyMode(s string) (DeletePropertyMode, error) {
	m := DeletePropertyMode(s)
	if !m.Valid() {
		return "", fmt.Errorf("%w: %q", ErrInvalidDeletePropertyMode, s)
	}
	return m, nil
}

func (m DeletePropertyMode) Valid() bool {
	switch m {
	case DeletePropertyModeCascade, DeletePropertyModeDetach:
		return true
	}
	return false
}
```

**Step 2: Commit**

```bash
git add apps/backend/internal/properties/domain/property.go
git commit -m "feat(properties): add DeletePropertyMode domain enum"
```

---

### Task 4: Add audit action constant

**Files:**
- Modify: `apps/backend/internal/audit/domain/entry.go`

**Step 1: Add constant**

After `ActionPropertyPhotoDeleted` add:

```go
ActionPropertyDeleted Action = "property.deleted"
```

**Step 2: Commit**

```bash
git add apps/backend/internal/audit/domain/entry.go
git commit -m "feat(audit): add property.deleted action"
```

---

### Task 5: Extend `PropertyRepository` port

**Files:**
- Modify: `apps/backend/internal/properties/application/ports.go`

**Step 1: Add Delete and cascade-delete methods to interface**

In `PropertyRepository` interface add after `CountActiveByOwner`:

```go
Delete(ctx context.Context, id, ownerID uuid.UUID) error
DeleteOperationsByProperty(ctx context.Context, ownerID, propertyID uuid.UUID) error
DeleteRecurringOperationsByProperty(ctx context.Context, ownerID, propertyID uuid.UUID) error
DeleteLeasesByProperty(ctx context.Context, ownerID, propertyID uuid.UUID) error
```

**Step 2: Commit**

```bash
git add apps/backend/internal/properties/application/ports.go
git commit -m "feat(properties): add delete and cascade-delete methods to repository port"
```

---

### Task 6: Implement repository Delete and cascade-delete methods

**Files:**
- Modify: `apps/backend/internal/properties/adapters/postgres/repository.go`

**Step 1: Implement methods**

Add after `CountActiveByOwner`:

```go
func (r *PropertyRepository) Delete(ctx context.Context, id, ownerID uuid.UUID) error {
	_, err := r.q().DeleteProperty(ctx, postgres.DeletePropertyParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(ownerID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return application.ErrNotFound
	}
	return err
}

func (r *PropertyRepository) DeleteOperationsByProperty(ctx context.Context, ownerID, propertyID uuid.UUID) error {
	return r.q().DeleteOperationsByProperty(ctx, postgres.DeleteOperationsByPropertyParams{
		OwnerID:    pgconv.UUIDToPgtype(ownerID),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	})
}

func (r *PropertyRepository) DeleteRecurringOperationsByProperty(ctx context.Context, ownerID, propertyID uuid.UUID) error {
	return r.q().DeleteRecurringOperationsByProperty(ctx, postgres.DeleteRecurringOperationsByPropertyParams{
		OwnerID:    pgconv.UUIDToPgtype(ownerID),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	})
}

func (r *PropertyRepository) DeleteLeasesByProperty(ctx context.Context, ownerID, propertyID uuid.UUID) error {
	return r.q().DeleteLeasesByProperty(ctx, postgres.DeleteLeasesByPropertyParams{
		OwnerID:    pgconv.UUIDToPgtype(ownerID),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	})
}
```

**Step 2: Run go vet on package**

Run:
```bash
cd apps/backend && go vet ./internal/properties/...
```
Expected: no errors.

**Step 3: Commit**

```bash
git add apps/backend/internal/properties/adapters/postgres/repository.go
git commit -m "feat(properties): implement property delete and cascade-delete in postgres adapter"
```

---

### Task 7: Implement `DeleteProperty` in application service

**Files:**
- Modify: `apps/backend/internal/properties/application/service.go`

**Step 1: Add service method**

After `ArchiveProperty` method, add:

```go
func (s *PropertyService) DeleteProperty(
	ctx context.Context,
	ownerID, id uuid.UUID,
	mode domain.DeletePropertyMode,
) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	repo := s.repo.WithTx(tx)

	if _, err := repo.GetByIDAndOwnerForUpdate(ctx, id, ownerID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("get property: %w", err)
	}

	occupied, err := s.occupancyProvider.WithTx(tx).IsOccupied(ctx, ownerID, id)
	if err != nil {
		return fmt.Errorf("check occupancy: %w", err)
	}
	if occupied {
		return ErrPropertyHasOpenLease
	}

	photos, err := s.photoRepo.GetByPropertyID(ctx, id)
	if err != nil {
		return fmt.Errorf("list photos: %w", err)
	}

	if mode == domain.DeletePropertyModeCascade {
		if err := repo.DeleteOperationsByProperty(ctx, ownerID, id); err != nil {
			return fmt.Errorf("delete operations: %w", err)
		}
		if err := repo.DeleteRecurringOperationsByProperty(ctx, ownerID, id); err != nil {
			return fmt.Errorf("delete recurring operations: %w", err)
		}
		if err := repo.DeleteLeasesByProperty(ctx, ownerID, id); err != nil {
			return fmt.Errorf("delete leases: %w", err)
		}
	}

	if err := repo.Delete(ctx, id, ownerID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("delete property: %w", err)
	}

	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &ownerID,
		ActorRole:  auditdomain.ActorRoleOwner,
		Action:     auditdomain.ActionPropertyDeleted,
		EntityType: auditdomain.EntityProperty,
		EntityID:   &id,
		Context:    map[string]any{"mode": string(mode)},
	}); err != nil {
		return fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	for _, photo := range photos {
		key := photoStorageKeyFromURL(photo.URL)
		if err := s.photoStorage.Delete(ctx, key); err != nil {
			s.logger.ErrorContext(ctx, "failed to delete property photo from storage",
				slog.String("photo_id", photo.ID.String()),
				slog.String("error", sanitizeError(err)),
			)
		}
	}

	return nil
}
```

**Important:** `photoStorageKeyFromURL` must be reused from existing photo deletion logic. Inspect `DeletePropertyPhoto` in the same file and use the same helper it uses to extract the S3 storage key from a photo URL. If the helper is unexported and not accessible, duplicate its logic inline in `DeleteProperty`.

**Step 2: Commit**

```bash
git add apps/backend/internal/properties/application/service.go
git commit -m "feat(properties): implement DeleteProperty service method"
```

---

### Task 8: Add HTTP handler

**Files:**
- Modify: `apps/backend/internal/platform/httpapi/property_handlers.go`

**Step 1: Add handler method**

Add after `UpdateProperty` handler:

```go
// DeleteProperty implements DELETE /properties/{id}.
func (h *PropertyHandlers) DeleteProperty(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "Некорректный идентификатор объекта"))
		return
	}

	modeStr := r.URL.Query().Get("mode")
	mode, err := domain.ParseDeletePropertyMode(modeStr)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "Некорректный режим удаления"))
		return
	}

	if err := h.svc.DeleteProperty(r.Context(), ownerID, id, mode); err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

**Step 2: Commit**

```bash
git add apps/backend/internal/platform/httpapi/property_handlers.go
git commit -m "feat(properties): add DeleteProperty HTTP handler"
```

---

### Task 9: Update OpenAPI contract

**Files:**
- Modify: `apps/backend/api/openapi/openapi.yaml`

**Step 1: Add DELETE operation to `/properties/{id}`**

After the existing `patch` block under `/properties/{id}`, add:

```yaml
    delete:
      operationId: deleteProperty
      security:
        - sessionCookie: []
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
            format: uuid
        - name: mode
          in: query
          required: true
          schema:
            type: string
            enum:
              - cascade
              - detach
      responses:
        '204':
          description: Property deleted
        '400':
          $ref: '#/components/responses/BadRequest'
        '401':
          $ref: '#/components/responses/Unauthorized'
        '404':
          $ref: '#/components/responses/NotFound'
        '409':
          $ref: '#/components/responses/Conflict'
        '403':
          $ref: '#/components/responses/SubscriptionBlocked'
```

**Step 2: Commit**

```bash
git add apps/backend/api/openapi/openapi.yaml
git commit -m "feat(api): add DELETE /properties/{id} OpenAPI contract"
```

---

### Task 10: Regenerate generated code

**Files:**
- Modify: `apps/backend/internal/platform/openapi/generated.gen.go`
- Modify: `apps/backend/internal/platform/generated/postgres/*.go` (if sqlc changed)

**Step 1: Regenerate OpenAPI server code**

Run:
```bash
cd apps/backend
go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.7.1 -config api/openapi/oapi-codegen.yaml api/openapi/openapi.yaml
```
Expected: `internal/platform/openapi/generated.gen.go` updates, no errors.

**Step 2: Verify generated handler method exists**

Run:
```bash
cd apps/backend && grep -n "DeleteProperty" internal/platform/openapi/generated.gen.go
```
Expected: at least one match for the new handler interface method.

**Step 3: Commit**

```bash
git add apps/backend/internal/platform/openapi/generated.gen.go \
        apps/backend/internal/platform/generated/postgres
git commit -m "chore(gen): regenerate openapi and sqlc code for property deletion"
```

---

### Task 11: Add Bruno requests

**Files:**
- Create: `tools/bruno/arenda-api/properties/delete property cascade.bru`
- Create: `tools/bruno/arenda-api/properties/delete property detach.bru`

**Step 1: Create cascade request**

```yaml
meta {
  name: delete property cascade
  type: http
  seq: 10
}

delete {
  url: {{baseUrl}}/properties/{{propertyId}}?mode=cascade
  body: none
  auth: inherit
}

params:query {
  mode: cascade
}
```

**Step 2: Create detach request**

```yaml
meta {
  name: delete property detach
  type: http
  seq: 11
}

delete {
  url: {{baseUrl}}/properties/{{propertyId}}?mode=detach
  body: none
  auth: inherit
}

params:query {
  mode: detach
}
```

**Step 3: Verify coverage script**

Run:
```bash
./tools/e2e/check-bruno-coverage.sh
```
Expected: exit 0, no missing DELETE /properties/{id} route.

**Step 4: Commit**

```bash
git add tools/bruno/arenda-api/properties
git commit -m "test(bruno): add property delete cascade and detach requests"
```

---

### Task 12: Run backend verification

**Files:**
- All changed files

**Step 1: Run gopls diagnostics**

Use `mcp__gopls__go_diagnostics` on changed Go files. Expected: 0 errors.

**Step 2: Run lint**

Run:
```bash
make backend-lint
```
Expected: exit 0.

**Step 3: Run tests**

Run:
```bash
cd apps/backend && go test ./...
```
Expected: all existing tests pass.

**Step 4: Run go vet**

Run:
```bash
cd apps/backend && go vet ./...
```
Expected: no issues.

**Step 5: Apply migrations locally and test via Bruno**

Run:
```bash
make migrate-up
```
Then use Bruno collection `tools/bruno/arenda-api/properties/delete property cascade.bru` and `delete property detach.bru` against a local backend. Verify in DB:
- cascade: `properties`, `leases`, `operations`, `recurring_operations`, `property_photos` rows for the property are gone.
- detach: `properties` row is gone, `property_photos` rows are gone, but `leases`, `operations`, `recurring_operations` still exist with `property_id = NULL`.
- open lease case: endpoint returns 409 and property remains.

**Step 6: Final commit**

```bash
git add -A
git commit -m "feat(properties): implement property deletion with cascade and detach modes"
```

---

## Notes

- Do not write new Go unit tests unless the user explicitly asks; verification is via Bruno and DB checks.
- For `photoStorageKeyFromURL`, reuse the same logic already present in `DeletePropertyPhoto` to extract the storage key from the photo URL.
- If `gopls` diagnostics report issues, fix them before running `make backend-lint`.
- The migration changes FK actions to `ON DELETE SET NULL`; therefore `detach` mode relies on the database to nullify `property_id`, and `cascade` mode must explicitly delete child records before the property.
