> **Note:** This file was produced by the `writing-plans` skill. It is both the design doc and the step-by-step implementation plan.

# Страница создания объекта — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans` to implement this plan task-by-task.

**Goal:** Перевести `/properties/new` с одноэкранной формы на пошаговый мастер по макетам Figma, с подсказками DaData, локальными превью фото, загрузкой фото в S3 после создания объекта и экраном успеха.

**Architecture:** Frontend-мастер на одном URL с `sessionStorage`; backend endpoint для DaData; backend endpoint для загрузки фото к объекту; S3-адаптер для REG.RU S3.

**Tech Stack:** Next.js 16 + React 19 + TypeScript + CSS Modules, `@heroui/react` v3 (Select/Input), TanStack Query; Go 1.26 + chi + sqlc + pgx + OpenAPI (oapi-codegen); REG.RU S3-compatible storage.

---

### Task 1: Обновить метку типа `parking`

**Files:**
- Modify: `apps/frontend/features/properties/lib/property-types.ts:12`

**Step 1: Изменить label**

```ts
parking: 'Машиноместо',
```

**Step 2: Проверить линтер**

Run: `cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend && npm run lint`
Expected: 0 errors, прежние warnings в auth.

**Step 3: Commit**

```bash
git add apps/frontend/features/properties/lib/property-types.ts
git commit -m "chore: rename parking label to match Figma"
```

---

### Task 2: Хук для черновика мастера

**Files:**
- Create: `apps/frontend/widgets/properties/lib/use-property-create-draft.ts`

**Step 1: Написать хук**

```ts
'use client';

import { useState, useEffect } from 'react';
import type { PropertyType } from '@/entities/property/model/types';

export type CreateStep = 1 | 2 | 3 | 4;

export type CreateDraft = {
  step: CreateStep;
  type?: PropertyType;
  address?: string;
  name?: string;
  description?: string;
};

const STORAGE_KEY = 'property-create-draft';

export function usePropertyCreateDraft() {
  const [draft, setDraft] = useState<CreateDraft>(() => loadDraft());

  useEffect(() => {
    if (draft.step === 4) {
      sessionStorage.removeItem(STORAGE_KEY);
      return;
    }
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify(draft));
  }, [draft]);

  return { draft, setDraft };
}

function loadDraft(): CreateDraft {
  if (typeof window === 'undefined') return { step: 1 };
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY);
    if (!raw) return { step: 1 };
    const parsed = JSON.parse(raw) as CreateDraft;
    if (parsed.step < 1 || parsed.step > 4) return { step: 1 };
    return parsed;
  } catch {
    return { step: 1 };
  }
}
```

**Step 2: Проверить линтер**

Run: `cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend && npm run lint`
Expected: 0 errors.

**Step 3: Commit**

```bash
git add apps/frontend/widgets/properties/lib/use-property-create-draft.ts
git commit -m "feat: add property create draft sessionStorage hook"
```

---

### Task 3: UI-скелет мастера и шапка

**Files:**
- Modify: `apps/frontend/app/(cabinet)/properties/new/page.tsx`
- Modify: `apps/frontend/app/(cabinet)/properties/new/page.module.css`
- Create: `apps/frontend/widgets/properties/ui/PropertyCreateWizard.tsx`
- Create: `apps/frontend/widgets/properties/ui/PropertyCreateWizard.module.css`
- Create: `apps/frontend/widgets/properties/ui/CreateObjectHeader.tsx`
- Create: `apps/frontend/widgets/properties/ui/CreateObjectHeader.module.css`
- Create: `apps/frontend/widgets/properties/ui/PropertyTypeStep.tsx`
- Create: `apps/frontend/widgets/properties/ui/PropertyAddressStep.tsx`
- Create: `apps/frontend/widgets/properties/ui/PropertyInfoStep.tsx`
- Create: `apps/frontend/widgets/properties/ui/PropertySuccessStep.tsx`

**Step 1: Шапка**

Создать `CreateObjectHeader.tsx`, вынести туда кнопки «Назад»/«Закрыть», бейдж шага и прогресс-бар.

**Step 2: Контейнер мастера**

Создать `PropertyCreateWizard.tsx`, который читает `draft`, рендерит текущий шаг и передаёт `onNext`/`onBack`.

**Step 3: Заглушки шагов**

Каждый шаг возвращает `<div>Step N</div>`.

**Step 4: Страница**

Заменить `PropertyCreateForm` на `PropertyCreateWizard`.

**Step 5: Проверить**

Run: `cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend && npm run build`
Expected: exit 0.

**Step 6: Commit**

```bash
git add apps/frontend/app/(cabinet)/properties/new/page.tsx apps/frontend/app/(cabinet)/properties/new/page.module.css apps/frontend/widgets/properties/ui/CreateObjectHeader.tsx apps/frontend/widgets/properties/ui/CreateObjectHeader.module.css apps/frontend/widgets/properties/ui/PropertyCreateWizard.tsx apps/frontend/widgets/properties/ui/PropertyCreateWizard.module.css apps/frontend/widgets/properties/ui/PropertyTypeStep.tsx apps/frontend/widgets/properties/ui/PropertyAddressStep.tsx apps/frontend/widgets/properties/ui/PropertyInfoStep.tsx apps/frontend/widgets/properties/ui/PropertySuccessStep.tsx
git commit -m "feat: scaffold property create wizard with header"
```

---

### Task 4: Шаг 1 — выбор типа

**Files:**
- Modify: `apps/frontend/widgets/properties/ui/PropertyTypeStep.tsx`
- Create: `apps/frontend/widgets/properties/ui/PropertyTypeStep.module.css`

**Step 1: Реализовать чипы**

Использовать `propertyTypeOptions` и проектный `Button`. Выбранный тип — `variant="primary"`, остальные — `variant="secondary"`.

**Step 2: Кнопка «Продолжить»**

`disabled={!type}`.

**Step 3: Проверить**

Run: `cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend && npm run lint`
Expected: 0 errors.

**Step 4: Commit**

```bash
git add apps/frontend/widgets/properties/ui/PropertyTypeStep.tsx apps/frontend/widgets/properties/ui/PropertyTypeStep.module.css
git commit -m "feat: property create type step"
```

---

### Task 5: Backend — OpenAPI-контракт DaData

**Files:**
- Modify: `apps/backend/api/openapi/openapi.yaml`

**Step 1: Добавить путь**

```yaml
  /dadata/suggestions/address:
    get:
      operationId: getAddressSuggestions
      security:
        - sessionCookie: []
      parameters:
        - name: query
          in: query
          required: true
          schema:
            type: string
            minLength: 3
            maxLength: 255
      responses:
        '200':
          description: Address suggestions
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/AddressSuggestionsResponse'
        '400':
          $ref: '#/components/responses/BadRequest'
        '401':
          $ref: '#/components/responses/Unauthorized'
        '500':
          $ref: '#/components/responses/InternalServerError'
```

**Step 2: Добавить схемы**

```yaml
    AddressSuggestionsResponse:
      type: object
      required: [suggestions]
      properties:
        suggestions:
          type: array
          items:
            $ref: '#/components/schemas/AddressSuggestion'
    AddressSuggestion:
      type: object
      required: [value]
      properties:
        value:
          type: string
        city:
          type: string
```

**Step 3: Сгенерировать типы**

Run:
```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/backend

go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.0 -config api/openapi/oapi-codegen.yaml api/openapi/openapi.yaml
```
Expected: `internal/platform/openapi/generated.gen.go` обновлён без ошибок.

**Step 4: Commit**

```bash
git add apps/backend/api/openapi/openapi.yaml apps/backend/internal/platform/openapi/generated.gen.go
git commit -m "feat(api): add dadata address suggestions contract"
```

---

### Task 6: Backend — DaData клиент и handler

**Files:**
- Modify: `apps/backend/internal/platform/config/config.go`
- Create: `apps/backend/internal/platform/dadata/client.go`
- Create: `apps/backend/internal/platform/httpapi/dadata_handlers.go`
- Modify: `apps/backend/internal/platform/httpapi/server.go`
- Modify: `apps/backend/cmd/api/main.go`
- Modify: `.env.example`

**Step 1: Добавить `DaDataAPIKey` в Config**

```go
DaDataAPIKey string
```

И загрузку `os.Getenv("DADATA_API_KEY")`.

**Step 2: Создать клиент DaData**

```go
package dadata

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "net/url"
    "strings"
    "time"
)

type Client struct {
    apiKey string
    baseURL string
    client *http.Client
}

func NewClient(apiKey string) *Client {
    return &Client{
        apiKey: apiKey,
        baseURL: "https://suggestions.dadata.ru/suggestions/api/4_1/rs/suggest/address",
        client: &http.Client{Timeout: 5 * time.Second},
    }
}

type Suggestion struct {
    Value string `json:"value"`
    Data  struct {
        City string `json:"city"`
    } `json:"data"`
}

func (c *Client) SuggestAddress(ctx context.Context, query string) ([]Suggestion, error) {
    payload := map[string]string{"query": query}
    body, _ := json.Marshal(payload)
    req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, strings.NewReader(string(body)))
    if err != nil { return nil, err }
    req.Header.Set("Authorization", "Token "+c.apiKey)
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Accept", "application/json")

    resp, err := c.client.Do(req)
    if err != nil { return nil, err }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("dadata returned %d", resp.StatusCode)
    }

    var result struct {
        Suggestions []Suggestion `json:"suggestions"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil { return nil, err }
    return result.Suggestions, nil
}
```

**Step 3: Handler**

```go
package httpapi

import (
    "net/http"
    "github.com/nambers/arenda-planform/apps/backend/internal/platform/dadata"
    "github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
)

type DadataHandlers struct {
    client *dadata.Client
}

func NewDadataHandlers(client *dadata.Client) *DadataHandlers {
    return &DadataHandlers{client: client}
}

func (h *DadataHandlers) GetAddressSuggestions(w http.ResponseWriter, r *http.Request, params openapi.GetAddressSuggestionsParams) {
    if params.Query == "" || len(params.Query) < 3 {
        writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "query too short"))
        return
    }
    suggestions, err := h.client.SuggestAddress(r.Context(), params.Query)
    if err != nil {
        writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
        return
    }
    items := make([]openapi.AddressSuggestion, 0, len(suggestions))
    for _, s := range suggestions {
        items = append(items, openapi.AddressSuggestion{Value: s.Value, City: s.Data.City})
    }
    writeJSON(r.Context(), w, http.StatusOK, openapi.AddressSuggestionsResponse{Suggestions: items})
}
```

**Step 4: Подключить в server.go**

Добавить `DadataHandlers` в `Deps`, в `composedHandler`.

**Step 5: Подключить в main.go**

```go
dadataClient := dadata.NewClient(cfg.DaDataAPIKey)
```

Передать в `httpapi.Deps`.

**Step 6: Обновить `.env.example`**

Раскомментировать `# DADATA_API_KEY=`.

**Step 7: Линтер и тесты**

Run:
```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform && make backend-lint
cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/backend && go test ./...
```
Expected: 0 failures.

**Step 8: Commit**

```bash
git add apps/backend/internal/platform/config/config.go apps/backend/internal/platform/dadata apps/backend/internal/platform/httpapi/dadata_handlers.go apps/backend/internal/platform/httpapi/server.go apps/backend/cmd/api/main.go .env.example
git commit -m "feat(backend): dadata address suggestions endpoint"
```

---

### Task 7: Frontend — хук для DaData

**Files:**
- Create: `apps/frontend/features/properties/api/use-dadata-suggestions.ts`

**Step 1: Написать хук**

```ts
'use client';

import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';

export type AddressSuggestion = {
  value: string;
  city?: string;
};

export function useDaDataSuggestions(query: string) {
  return useQuery<AddressSuggestion[]>({
    queryKey: ['dadata', 'address', query],
    queryFn: async () => {
      const params = new URLSearchParams({ query });
      return apiClient<AddressSuggestion[]>(`/dadata/suggestions/address?${params.toString()}`);
    },
    enabled: query.length >= 3,
    staleTime: 60_000,
  });
}
```

**Step 2: Проверить**

Run: `cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend && npm run lint`
Expected: 0 errors.

**Step 3: Commit**

```bash
git add apps/frontend/features/properties/api/use-dadata-suggestions.ts
git commit -m "feat: dadata suggestions hook"
```

---

### Task 8: Шаг 2 — ввод адреса

**Files:**
- Modify: `apps/frontend/widgets/properties/ui/PropertyAddressStep.tsx`
- Create: `apps/frontend/widgets/properties/ui/PropertyAddressStep.module.css`

**Step 1: Реализовать поле с автодополнением**

Использовать проектный `TextField` + выпадающий список кнопок. При выборе подсказки сохранять `value` в `draft.address`.

**Step 2: Кнопка «Продолжить»**

`disabled={!address}`.

**Step 3: Проверить**

Run: `cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend && npm run lint`
Expected: 0 errors.

**Step 4: Commit**

```bash
git add apps/frontend/widgets/properties/ui/PropertyAddressStep.tsx apps/frontend/widgets/properties/ui/PropertyAddressStep.module.css
git commit -m "feat: property create address step"
```

---

### Task 9: Backend — схема для фото

**Files:**
- Create: `apps/backend/db/migrations/000038_property_photos.up.sql`
- Create: `apps/backend/db/migrations/000038_property_photos.down.sql`
- Modify: `apps/backend/db/queries/properties.sql`

**Step 1: Миграция**

```sql
CREATE TABLE property_photos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    property_id UUID NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    storage_key TEXT NOT NULL,
    url TEXT NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_property_photos_property_id ON property_photos(property_id);
```

**Step 2: Down-миграция**

```sql
DROP TABLE IF EXISTS property_photos;
```

**Step 3: Queries**

```sql
-- name: CreatePropertyPhoto :one
INSERT INTO property_photos (property_id, storage_key, url, sort_order)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListPropertyPhotosByPropertyID :many
SELECT * FROM property_photos
WHERE property_id = $1
ORDER BY sort_order ASC, created_at ASC;

-- name: DeletePropertyPhoto :one
DELETE FROM property_photos
WHERE id = $1 AND property_id = $2
RETURNING *;
```

**Step 4: Сгенерировать sqlc**

Run:
```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/backend
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
```
Expected: `internal/platform/generated/postgres/properties.sql.go` обновлён.

**Step 5: Commit**

```bash
git add apps/backend/db/migrations/000038_property_photos.* apps/backend/db/queries/properties.sql apps/backend/internal/platform/generated/postgres/properties.sql.go
git commit -m "feat(db): property_photos table and queries"
```

---

### Task 10: Backend — S3 storage adapter

**Files:**
- Modify: `apps/backend/internal/platform/config/config.go`
- Create: `apps/backend/internal/platform/storage/port.go`
- Create: `apps/backend/internal/platform/storage/s3.go`
- Modify: `.env.example`

**Step 1: Config**

```go
type S3Config struct {
    Endpoint       string
    Region         string
    Bucket         string
    AccessKey      string
    SecretKey      string
    PublicBaseURL  string
    PathStyle      bool
}
```

Добавить в `Config` поле `S3 S3Config` и загрузку переменных `REGRU_S3_*`.

**Step 2: Port**

```go
package storage

import "context"

type UploadResult struct {
    Key string
    URL string
}

type Uploader interface {
    Upload(ctx context.Context, key string, data []byte, contentType string) (UploadResult, error)
}
```

**Step 3: S3-реализация**

Использовать `github.com/aws/aws-sdk-go-v2` (добавить в `go.mod`).

**Step 4: Commit**

```bash
git add apps/backend/internal/platform/config/config.go apps/backend/internal/platform/storage apps/backend/go.mod apps/backend/go.sum .env.example
git commit -m "feat(backend): reg.ru s3 storage adapter"
```

---

### Task 11: Backend — endpoint загрузки фото

**Files:**
- Modify: `apps/backend/api/openapi/openapi.yaml`
- Modify: `apps/backend/internal/properties/application/ports.go`
- Modify: `apps/backend/internal/properties/application/service.go`
- Modify: `apps/backend/internal/properties/adapters/postgres/repository.go`
- Create: `apps/backend/internal/properties/adapters/photos/s3.go`
- Modify: `apps/backend/internal/platform/httpapi/property_handlers.go`
- Modify: `apps/backend/internal/platform/httpapi/server.go`
- Modify: `apps/backend/cmd/api/main.go`

**Step 1: OpenAPI**

Добавить:
```yaml
  /properties/{propertyId}/photos:
    post:
      operationId: uploadPropertyPhotos
      security:
        - sessionCookie: []
      parameters:
        - name: propertyId
          in: path
          required: true
          schema:
            type: string
            format: uuid
      requestBody:
        required: true
        content:
          multipart/form-data:
            schema:
              type: object
              properties:
                photos:
                  type: array
                  items:
                    type: string
                    format: binary
                  maxItems: 10
      responses:
        '201':
          description: Photos uploaded
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/PropertyPhotosResponse'
```

```yaml
    PropertyPhotosResponse:
      type: object
      required: [items]
      properties:
        items:
          type: array
          items:
            type: string
```

**Step 2: Application**

Добавить в `PropertyRepository` методы: `CreatePhoto`, `ListPhotosByPropertyID`, `DeletePhoto`.

Добавить в `PropertyService`:

```go
func (s *PropertyService) UploadPhotos(ctx context.Context, ownerID, propertyID uuid.UUID, files []FileUpload) ([]string, error)
```

Проверить владельца, загрузить в S3, сохранить в БД.

**Step 3: Handler**

Реализовать `UploadPropertyPhotos` в `property_handlers.go`.

**Step 4: Подключить**

Добавить `storage.Uploader` в `Deps` и `PropertyService`.

**Step 5: Регенерировать OpenAPI**

Run:
```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/backend

go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.0 -config api/openapi/oapi-codegen.yaml api/openapi/openapi.yaml
```

**Step 6: Линтер и тесты**

Run:
```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform && make backend-lint
cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/backend && go test ./...
```
Expected: 0 failures.

**Step 7: Commit**

```bash
git add apps/backend/api/openapi/openapi.yaml apps/backend/internal/properties apps/backend/internal/platform/httpapi/property_handlers.go apps/backend/internal/platform/httpapi/server.go apps/backend/cmd/api/main.go apps/backend/internal/platform/openapi/generated.gen.go
git commit -m "feat(backend): upload property photos endpoint"
```

---

### Task 12: Backend — возвращать фото в PropertyResponse

**Files:**
- Modify: `apps/backend/api/openapi/openapi.yaml`
- Modify: `apps/backend/internal/platform/httpapi/property_handlers.go`
- Modify: `apps/backend/internal/properties/application/service.go`
- Modify: `apps/backend/internal/properties/application/ports.go`
- Modify: `apps/backend/internal/properties/adapters/postgres/repository.go`

**Step 1: OpenAPI**

В `PropertyResponse` добавить:
```yaml
        photos:
          type: array
          items:
            type: string
```

**Step 2: Service**

В `GetProperty`, `ListProperties` загружать фото через `ListPhotosByPropertyID` и маппить в `[]string` URL.

**Step 3: Handler**

Обновить `propertyResponse` mapper.

**Step 4: Регенерация и тесты**

Run:
```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/backend

go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.0 -config api/openapi/oapi-codegen.yaml api/openapi/openapi.yaml
go test ./...
```
Expected: 0 failures.

**Step 5: Commit**

```bash
git add apps/backend/api/openapi/openapi.yaml apps/backend/internal/platform/openapi/generated.gen.go apps/backend/internal/properties apps/backend/internal/platform/httpapi/property_handlers.go
git commit -m "feat(backend): include photos in property response"
```

---

### Task 13: Frontend — обновить сгенерированные типы

**Files:**
- Modify: `apps/frontend/shared/api/generated.ts`

**Step 1: Сгенерировать**

Run:
```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend && npm run generate:api
```
Expected: `shared/api/generated.ts` обновлён.

**Step 2: Commit**

```bash
git add apps/frontend/shared/api/generated.ts
git commit -m "feat: regenerate api types with dadata and photos"
```

---

### Task 14: Frontend — хук для фото

**Files:**
- Create: `apps/frontend/widgets/properties/lib/use-property-photos.ts`
- Create: `apps/frontend/features/properties/api/use-upload-property-photos.ts`

**Step 1: Локальные фото**

```ts
'use client';

import { useState, useCallback } from 'react';

const MAX_FILES = 10;
const MAX_SIZE = 5 * 1024 * 1024;
const ALLOWED_TYPES = ['image/jpeg', 'image/png', 'image/webp'];

export type PhotoFile = {
  file: File;
  preview: string;
  id: string;
};

export type PhotoError = {
  fileName: string;
  reason: 'type' | 'size';
};

export function usePropertyPhotos(initial: PhotoFile[] = []) {
  const [photos, setPhotos] = useState<PhotoFile[]>(initial);
  const [errors, setErrors] = useState<PhotoError[]>([]);

  const addFiles = useCallback((files: FileList | null) => {
    if (!files) return;
    setErrors([]);
    const newPhotos: PhotoFile[] = [];
    const newErrors: PhotoError[] = [];
    Array.from(files).slice(0, MAX_FILES - photos.length).forEach((file) => {
      if (!ALLOWED_TYPES.includes(file.type)) {
        newErrors.push({ fileName: file.name, reason: 'type' });
        return;
      }
      if (file.size > MAX_SIZE) {
        newErrors.push({ fileName: file.name, reason: 'size' });
        return;
      }
      newPhotos.push({ file, preview: URL.createObjectURL(file), id: crypto.randomUUID() });
    });
    setPhotos((prev) => [...prev, ...newPhotos].slice(0, MAX_FILES));
    setErrors(newErrors);
  }, [photos.length]);

  const removePhoto = useCallback((id: string) => {
    setPhotos((prev) => {
      const removed = prev.find((p) => p.id === id);
      if (removed) URL.revokeObjectURL(removed.preview);
      return prev.filter((p) => p.id !== id);
    });
  }, []);

  return { photos, errors, addFiles, removePhoto };
}
```

**Step 2: Загрузка**

```ts
'use client';

import { useMutation } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';

export function useUploadPropertyPhotos() {
  return useMutation({
    mutationFn: async ({ propertyId, files }: { propertyId: string; files: File[] }) => {
      const formData = new FormData();
      files.forEach((file) => formData.append('photos', file));
      return apiClient<string[]>(`/properties/${propertyId}/photos`, {
        method: 'POST',
        body: formData,
      });
    },
  });
}
```

**Step 3: Проверить**

Run: `cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend && npm run lint`
Expected: 0 errors.

**Step 4: Commit**

```bash
git add apps/frontend/widgets/properties/lib/use-property-photos.ts apps/frontend/features/properties/api/use-upload-property-photos.ts
git commit -m "feat: property photo upload hooks"
```

---

### Task 15: Шаг 3 — информация и фото

**Files:**
- Modify: `apps/frontend/widgets/properties/ui/PropertyInfoStep.tsx`
- Create: `apps/frontend/widgets/properties/ui/PropertyInfoStep.module.css`

**Step 1: Поля**

- `TextField` «Название» обязательное.
- `TextField` «Описание» многострочное, `maxLength={500}`.

**Step 2: Блок фото**

- Сетка превью 96×96 с кнопкой удаления.
- Кнопка «Добавить фото» / «Загружено максимум».
- Подпись «N из 10 фото».

**Step 3: Кнопка «Создать объект»**

`disabled={!name || create.isPending}`.

**Step 4: Проверить**

Run: `cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend && npm run build`
Expected: exit 0.

**Step 5: Commit**

```bash
git add apps/frontend/widgets/properties/ui/PropertyInfoStep.tsx apps/frontend/widgets/properties/ui/PropertyInfoStep.module.css
git commit -m "feat: property create info and photos step"
```

---

### Task 16: Шаг 4 — экран успеха

**Files:**
- Modify: `apps/frontend/widgets/properties/ui/PropertySuccessStep.tsx`
- Create: `apps/frontend/widgets/properties/ui/PropertySuccessStep.module.css`

**Step 1: Реализовать**

- 3D-логотип из `public/images/empty-logo.png`.
- Заголовок «Объект создан».
- Подзаголовок «Теперь вы можете создать и отслеживать аренду».
- Кнопки «Создать аренду» → `/tenants`, «Добавить позже» → `/properties`.

**Step 2: Проверить**

Run: `cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend && npm run build`
Expected: exit 0.

**Step 3: Commit**

```bash
git add apps/frontend/widgets/properties/ui/PropertySuccessStep.tsx apps/frontend/widgets/properties/ui/PropertySuccessStep.module.css
git commit -m "feat: property create success step"
```

---

### Task 17: Связать мастер и удалить старую форму

**Files:**
- Modify: `apps/frontend/widgets/properties/ui/PropertyCreateWizard.tsx`
- Modify: `apps/frontend/widgets/properties/ui/index.ts`
- Delete: `apps/frontend/widgets/properties/ui/PropertyCreateForm.tsx`
- Delete: `apps/frontend/widgets/properties/ui/PropertyCreateForm.module.css`

**Step 1: Wizard**

Подключить шаги, передавать `draft`, `setDraft`, `onNext`, `onBack`, `onSubmit`.

**Step 2: Submit**

```ts
const handleSubmit = async () => {
  const created = await create.mutateAsync({ name, type, address, description });
  if (photos.length > 0) {
    await upload.mutateAsync({ propertyId: created.id, files: photos.map(p => p.file) });
  }
  setDraft({ step: 4 });
};
```

**Step 3: Экспорты**

Обновить `widgets/properties/ui/index.ts`.

**Step 4: Проверить**

Run:
```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend && npm run lint && npm run build
```
Expected: 0 errors, exit 0.

**Step 5: Commit**

```bash
git add apps/frontend/widgets/properties/ui/PropertyCreateWizard.tsx apps/frontend/widgets/properties/ui/index.ts
git rm apps/frontend/widgets/properties/ui/PropertyCreateForm.tsx apps/frontend/widgets/properties/ui/PropertyCreateForm.module.css
git commit -m "feat: wire property create wizard and remove old form"
```

---

### Task 18: Адаптив и полировка

**Files:**
- Modify: `apps/frontend/widgets/properties/ui/*.module.css`

**Step 1: Проверить breakpoints**

- Десктоп: max-width 560px, центрирование.
- Планшет: padding 84px 40px.
- Мобильный: padding 84px 20px.

**Step 2: Скриншоты**

Run:
```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend
make local-infra-up
make backend-run
npm run dev
```

Открыть `http://localhost:3000/properties/new`, сделать скриншоты каждого шага и экрана успеха.

**Step 3: Commit**

```bash
git add -A
git commit -m "style: responsive polish for property create wizard"
```

---

### Task 19: CHANGELOG

**Files:**
- Modify: `CHANGELOG.md`

**Step 1: Добавить запись**

```markdown
### Добавлено
- Страница создания объекта стала пошаговым мастером: выбор типа, адрес с подсказками DaData, информация и фотографии, экран успеха.
```

**Step 2: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs: update changelog"
```

---

### Task 20: Финальные проверки

**Step 1: Backend**

Run:
```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform && make backend-lint
cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/backend && go test ./...
```
Expected: 0 errors, 0 failures.

**Step 2: Frontend**

Run:
```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend && npm run lint && npm run build
```
Expected: 0 errors, exit 0.

**Step 3: E2E smoke**

Run:
```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend
# вручную пройти мастер в браузере
```

Expected: объект создаётся, фото загружаются, экран успеха отображается.
