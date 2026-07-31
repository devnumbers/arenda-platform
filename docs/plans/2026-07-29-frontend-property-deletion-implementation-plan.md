# Frontend Property Deletion Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a "Delete property" option to the property details page top menu, opening a modal that lets the user choose `cascade` or `detach` deletion, and blocks the action with lease info when an open lease exists.

**Architecture:** A single adaptive `PropertyDeleteModal` switches between mode-selection and lease-block states. A new `useDeleteProperty` mutation in the properties feature calls `DELETE /properties/{id}?mode=`. `PropertyDetailPage` orchestrates modal state, mutation, success redirect, and reuse of the existing `PropertyEndLeaseModal`.

**Tech Stack:** Next.js 16, React 19, TypeScript 5, HeroUI v3, TanStack Query, FSD.

---

### Task 1: Regenerate the API client

**Files:**
- Modify: `apps/frontend/shared/api/generated.ts`

**Step 1: Run the generator**

```bash
cd apps/frontend
npm run generate:api
```

Expected: `generated.ts` updates and now contains types/paths for `deleteProperty` (`/properties/{id}` DELETE).

**Step 2: Verify `deleteProperty` exists**

Run:
```bash
cd apps/frontend && grep -n "deleteProperty\|DeleteProperty" shared/api/generated.ts
```
Expected: matches for operation/path types.

**Step 3: Commit**

```bash
git add apps/frontend/shared/api/generated.ts
git commit -m "chore(frontend): regenerate api client for property deletion"
```

---

### Task 2: Add `useDeleteProperty` hook

**Files:**
- Modify: `apps/frontend/features/properties/api/hooks.ts`
- Modify: `apps/frontend/features/properties/api/index.ts`

**Step 1: Append hook to hooks.ts**

After `useDeletePropertyPhoto` add:

```typescript
export function useDeleteProperty(): UseMutationResult<
  void,
  ApiError,
  { id: string; mode: 'cascade' | 'detach' }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, mode }) =>
      apiClient<void>(`/properties/${id}?mode=${mode}`, {
        method: 'DELETE',
      }),
    onSuccess: (_, { id }) => {
      queryClient.invalidateQueries({ queryKey: propertyKeys.list });
      queryClient.removeQueries({ queryKey: propertyKeys.detail(id) });
      queryClient.invalidateQueries({ queryKey: leaseKeys.lists() });
      queryClient.invalidateQueries({ queryKey: operationKeys.lists() });
      queryClient.invalidateQueries({ queryKey: operationKeys.infiniteLists() });
      queryClient.invalidateQueries({ queryKey: operationKeys.byProperty(id) });
      queryClient.invalidateQueries({ queryKey: operationKeys.summary(id) });
    },
  });
}
```

**Note:** Add `leaseKeys` import if not present:
```typescript
import { leaseKeys } from '@/features/leases/api/keys';
```

**Step 2: Export from index.ts**

If `apps/frontend/features/properties/api/index.ts` re-exports hooks, add:
```typescript
export { useDeleteProperty } from './hooks';
```

**Step 3: Run typecheck on the feature**

```bash
cd apps/frontend && npx tsc --noEmit -p tsconfig.json
```
Expected: no errors.

**Step 4: Commit**

```bash
git add apps/frontend/features/properties/api/hooks.ts \
        apps/frontend/features/properties/api/index.ts
git commit -m "feat(properties): add useDeleteProperty mutation hook"
```

---

### Task 3: Add notification scenarios

**Files:**
- Modify: `apps/frontend/shared/lib/notifications/scenarios.ts`

**Step 1: Extend property scenarios**

In the `property` object add:

```typescript
deleted: ((options?) =>
  notify.success('Объект удалён', options)) satisfies ScenarioFn,
deleteError: errorScenario('Не удалось удалить объект'),
```

Place after `updated` and before `saveError`.

**Step 2: Run typecheck**

```bash
cd apps/frontend && npx tsc --noEmit -p tsconfig.json
```
Expected: no errors.

**Step 3: Commit**

```bash
git add apps/frontend/shared/lib/notifications/scenarios.ts
git commit -m "feat(notifications): add property deleted and deleteError scenarios"
```

---

### Task 4: Create `PropertyDeleteModal`

**Files:**
- Create: `apps/frontend/widgets/property-detail/ui/PropertyDeleteModal.tsx`
- Create: `apps/frontend/widgets/property-detail/ui/PropertyDeleteModal.module.css`

**Step 1: Create the modal component**

```tsx
'use client';

import type { JSX } from 'react';
import { Modal } from '@heroui/react';
import { Button } from '@/shared/ui/button';
import type { components } from '@/shared/api/generated';
import styles from './PropertyDeleteModal.module.css';

type Lease = components['schemas']['LeaseResponse'];

export type PropertyDeleteModalProps = {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onDelete: (mode: 'cascade' | 'detach') => void;
  readonly onEndLease: () => void;
  readonly currentLease?: Lease;
  readonly isPending: boolean;
};

export function PropertyDeleteModal({
  isOpen,
  onClose,
  onDelete,
  onEndLease,
  currentLease,
  isPending,
}: PropertyDeleteModalProps): JSX.Element {
  const handleOpenChange = (open: boolean): void => {
    if (!open) {
      onClose();
    }
  };

  return (
    <Modal isOpen={isOpen} onOpenChange={handleOpenChange}>
      <Modal.Backdrop>
        <Modal.Container placement="center" size="sm">
          <Modal.Dialog aria-label={currentLease ? 'Нельзя удалить объект' : 'Удалить объект?'}>
            <Modal.Header>
              <Modal.Heading>
                {currentLease ? 'Нельзя удалить объект' : 'Удалить объект?'}
              </Modal.Heading>
            </Modal.Header>
            <Modal.Body>
              {currentLease ? (
                <div className={styles.leaseBlock}>
                  <p>Объект нельзя удалить, пока есть незавершённая аренда.</p>
                  <p className={styles.leaseInfo}>
                    {currentLease.tenant_contact?.name ?? 'Арендатор не указан'}
                    {currentLease.start_date && ` · с ${currentLease.start_date}`}
                    {currentLease.end_date && ` по ${currentLease.end_date}`}
                  </p>
                </div>
              ) : (
                <p>
                  Вы хотите удалить все данные, связанные с объектом (аренды, операции) или только сам объект?
                </p>
              )}
            </Modal.Body>
            <Modal.Footer>
              {currentLease ? (
                <>
                  <Button variant="secondary" onClick={onClose} type="button">
                    Отмена
                  </Button>
                  <Button variant="primary" onClick={onEndLease} type="button">
                    Завершить аренду
                  </Button>
                </>
              ) : (
                <>
                  <Button variant="secondary" onClick={onClose} type="button">
                    Отмена
                  </Button>
                  <Button
                    variant="primary"
                    onClick={() => onDelete('detach')}
                    isLoading={isPending}
                    type="button"
                  >
                    Удалить только объект
                  </Button>
                  <Button
                    variant="primary"
                    onClick={() => onDelete('cascade')}
                    isLoading={isPending}
                    type="button"
                  >
                    Удалить всё
                  </Button>
                </>
              )}
            </Modal.Footer>
          </Modal.Dialog>
        </Modal.Container>
      </Modal.Backdrop>
    </Modal>
  );
}
```

**Step 2: Create CSS module**

```css
.leaseBlock {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.leaseInfo {
  color: var(--color-text-secondary);
}
```

**Step 3: Run typecheck**

```bash
cd apps/frontend && npx tsc --noEmit -p tsconfig.json
```
Expected: no errors.

**Step 4: Commit**

```bash
git add apps/frontend/widgets/property-detail/ui/PropertyDeleteModal.tsx \
        apps/frontend/widgets/property-detail/ui/PropertyDeleteModal.module.css
git commit -m "feat(property-detail): add PropertyDeleteModal"
```

---

### Task 5: Update `PropertyActionMenu`

**Files:**
- Modify: `apps/frontend/widgets/property-detail/ui/PropertyActionMenu.tsx`

**Step 1: Add `onDelete` prop and menu item**

Update props:
```typescript
export type PropertyActionMenuProps = {
  readonly status?: PropertyStatus;
  readonly disabled?: boolean;
  readonly onEdit: () => void;
  readonly onToggleMaintenance: () => void;
  readonly onToggleArchive: () => void;
  readonly onDelete: () => void;
};
```

Update destructuring:
```typescript
export function PropertyActionMenu({
  status,
  disabled,
  onEdit,
  onToggleMaintenance,
  onToggleArchive,
  onDelete,
}: PropertyActionMenuProps): JSX.Element {
```

Add delete option after archive option:
```typescript
items.push({
  value: 'delete',
  label: 'Удалить объект',
});
```

Add case in switch:
```typescript
case 'delete':
  onDelete();
  break;
```

**Step 2: Run typecheck**

```bash
cd apps/frontend && npx tsc --noEmit -p tsconfig.json
```
Expected: no errors.

**Step 3: Commit**

```bash
git add apps/frontend/widgets/property-detail/ui/PropertyActionMenu.tsx
git commit -m "feat(property-detail): add delete option to PropertyActionMenu"
```

---

### Task 6: Wire modal and mutation in `PropertyDetailPage`

**Files:**
- Modify: `apps/frontend/widgets/property-detail/ui/PropertyDetailPage.tsx`

**Step 1: Import new dependencies**

Add to imports:
```typescript
import { useDeleteProperty } from '@/features/properties/api/hooks';
import { goBack } from '@/shared/lib/navigation';
import { PropertyDeleteModal } from './PropertyDeleteModal';
```

**Step 2: Add mutation and modal state**

After `const completeLease = useCompleteLease();` add:
```typescript
const deleteProperty = useDeleteProperty();
```

After `const [selectedLeaseId, setSelectedLeaseId] = useState<string>('');` add:
```typescript
const [deleteModalOpen, setDeleteModalOpen] = useState(false);
```

**Step 3: Add handlers**

After `handleEdit` add:
```typescript
const handleDelete = useCallback((mode: 'cascade' | 'detach') => {
  deleteProperty.mutate(
    { id, mode },
    {
      onSuccess: () => {
        notify.scenarios.property.deleted();
        goBack(router, ROUTES.properties);
      },
      onError: showMutationError,
    },
  );
}, [deleteProperty, id, router]);

const handleDeleteEndLease = useCallback(() => {
  setDeleteModalOpen(false);
  handleEndLease();
}, [handleEndLease]);
```

**Step 4: Pass `onDelete` to `PropertyActionMenu`**

```typescript
<PropertyActionMenu
  status={property?.status}
  disabled={isLoading || hasAnyError || !property}
  onEdit={handleEdit}
  onToggleMaintenance={handleToggleMaintenance}
  onToggleArchive={handleToggleArchive}
  onDelete={() => setDeleteModalOpen(true)}
/>
```

**Step 5: Render modal**

After `PropertyEndLeaseModal` add:
```typescript
<PropertyDeleteModal
  isOpen={deleteModalOpen}
  onClose={() => setDeleteModalOpen(false)}
  onDelete={handleDelete}
  onEndLease={handleDeleteEndLease}
  currentLease={currentLease}
  isPending={deleteProperty.isPending}
/>
```

**Step 6: Run typecheck and lint**

```bash
cd apps/frontend && npx tsc --noEmit -p tsconfig.json
```
Expected: no errors.

```bash
cd apps/frontend && npm run lint
```
Expected: no errors.

**Step 7: Commit**

```bash
git add apps/frontend/widgets/property-detail/ui/PropertyDetailPage.tsx
git commit -m "feat(property-detail): wire property delete modal and mutation"
```

---

### Task 7: Build and final verification

**Files:**
- All changed frontend files

**Step 1: Build**

```bash
cd apps/frontend && npm run build
```
Expected: build succeeds.

**Step 2: Lint**

```bash
cd apps/frontend && npm run lint
```
Expected: 0 issues.

**Step 3: Manual verification**

Start dev server:
```bash
cd apps/frontend && npm run dev
```

Check in browser:
1. Open a property without an open lease.
2. Click top menu → «Удалить объект».
3. Modal shows two buttons: «Удалить только объект» and «Удалить всё».
4. Click «Удалить только объект» → redirect to properties list, object gone, related leases/operations survive with `property_id = NULL`.
5. Repeat with «Удалить всё» → object and related data removed.
6. Open a property with an active lease.
7. Click «Удалить объект» → modal shows lease info and «Завершить аренду» button.
8. Click «Завершить аренду» → existing end-lease modal opens.
9. Complete lease, reopen delete modal → now shows mode selection.
10. Click «Отмена» → modal closes without request.

**Step 4: Final commit**

```bash
git add -A
git commit -m "feat(property-detail): implement property deletion UI"
```

---

## Notes

- Do not write new unit tests unless the user explicitly asks; manual browser verification is the gate.
- `goBack(router, ROUTES.properties)` is used because deletion is a flow-ending action; per project convention, do not use `router.replace` here.
- The `currentLease` value is derived from the already-loaded leases list, so no extra request is needed.
