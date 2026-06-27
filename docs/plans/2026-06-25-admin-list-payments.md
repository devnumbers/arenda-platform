> I'm using the writing-plans skill to create the implementation plan.

# Admin List Payments Endpoint Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an admin-only `GET /admin/subscription/payments` endpoint that returns all subscription payments with optional status filtering, limit/offset pagination, and the owning user's phone number.

**Architecture:** Extend the existing billing application service and repository layers with an admin list method, expose it through the generated OpenAPI contract, protect the route with the existing `AdminOnlyMiddleware`, and reuse the existing `SubscriptionHandlers` type.

**Tech Stack:** Go, chi, oapi-codegen, sqlc, PostgreSQL, Bruno.

---

### Task 1: Add admin SQL queries

**Files:**
- Modify: `apps/backend/db/queries/billing.sql`

**Step 1: Append list query**

```sql
-- name: ListSubscriptionPaymentsAdmin :many
SELECT sp.*, u.phone AS user_phone
FROM subscription_payments sp
JOIN users u ON sp.user_id = u.id
WHERE ($1::text = '' OR sp.status = $1::text)
ORDER BY sp.created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountSubscriptionPaymentsAdmin :one
SELECT COUNT(*)
FROM subscription_payments sp
WHERE ($1::text = '' OR sp.status = $1::text);
```

`$1::text = ''` means "no status filter applied".

**Step 2: Regenerate sqlc code**

Run:

```bash
cd apps/backend
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
```

Expected: `internal/platform/generated/postgres/billing.sql.go` now contains `ListSubscriptionPaymentsAdmin` and `CountSubscriptionPaymentsAdmin`.

---

### Task 2: Add repository method

**Files:**
- Modify: `apps/backend/internal/billing/application/ports.go`
- Modify: `apps/backend/internal/billing/application/dto.go`
- Modify: `apps/backend/internal/billing/adapters/postgres/subscription_payment_repository.go`

**Step 1: Define DTO in `dto.go`**

```go
// SubscriptionPaymentWithUser is a subscription payment together with its tariff and the user's phone.
type SubscriptionPaymentWithUser struct {
	Payment   domain.SubscriptionPayment
	UserPhone string
}
```

**Step 2: Extend repository interface in `ports.go`**

Add to `SubscriptionPaymentRepository`:

```go
ListAll(ctx context.Context, status string, limit, offset int) ([]SubscriptionPaymentWithUser, int64, error)
```

**Step 3: Implement in `subscription_payment_repository.go`**

```go
// ListAll returns all subscription payments for admin view.
func (r *SubscriptionPaymentRepository) ListAll(ctx context.Context, status string, limit, offset int) ([]application.SubscriptionPaymentWithUser, int64, error) {
	rows, err := r.q().ListSubscriptionPaymentsAdmin(ctx, postgres.ListSubscriptionPaymentsAdminParams{
		Status: status,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list subscription payments admin: %w", err)
	}

	items := make([]application.SubscriptionPaymentWithUser, 0, len(rows))
	for _, row := range rows {
		p, err := mapSubscriptionPayment(row.SubscriptionPayment)
		if err != nil {
			return nil, 0, fmt.Errorf("map subscription payment: %w", err)
		}
		items = append(items, application.SubscriptionPaymentWithUser{
			Payment:   p,
			UserPhone: row.UserPhone,
		})
	}

	total, err := r.q().CountSubscriptionPaymentsAdmin(ctx, status)
	if err != nil {
		return nil, 0, fmt.Errorf("count subscription payments admin: %w", err)
	}

	return items, total, nil
}
```

**Step 4: Ensure it compiles**

Run:

```bash
cd apps/backend && go build ./internal/billing/...
```

Expected: OK.

---

### Task 3: Add application service method

**Files:**
- Modify: `apps/backend/internal/billing/application/service.go`

**Step 1: Add filters struct and service method**

```go
// ListAllPaymentsFilters carries optional filters for the admin list endpoint.
type ListAllPaymentsFilters struct {
	Status string
	Limit  int
	Offset int
}

// ListAllPayments returns all subscription payments for admin view.
func (s *BillingService) ListAllPayments(ctx context.Context, filters ListAllPaymentsFilters) ([]AdminSubscriptionPaymentView, int64, error) {
	if filters.Limit <= 0 {
		filters.Limit = 20
	}
	if filters.Limit > 100 {
		filters.Limit = 100
	}
	if filters.Offset < 0 {
		filters.Offset = 0
	}

	if filters.Status != "" &&
		filters.Status != string(domain.PaymentStatusPending) &&
		filters.Status != string(domain.PaymentStatusSucceeded) &&
		filters.Status != string(domain.PaymentStatusFailed) &&
		filters.Status != string(domain.PaymentStatusRefunded) &&
		filters.Status != string(domain.PaymentStatusPartialRefunded) {
		return nil, 0, fmt.Errorf("%w: invalid status filter", domain.ErrInvalidAmount)
	}

	payments, total, err := s.subscriptionPayments.ListAll(ctx, filters.Status, filters.Limit, filters.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list all payments: %w", err)
	}

	tariffs, err := s.tariffs.List(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list tariffs: %w", err)
	}
	tariffByID := make(map[uuid.UUID]domain.Tariff, len(tariffs))
	for _, t := range tariffs {
		tariffByID[t.ID] = t
	}

	views := make([]AdminSubscriptionPaymentView, 0, len(payments))
	for _, p := range payments {
		t, ok := tariffByID[p.Payment.TariffID]
		if !ok {
			return nil, 0, fmt.Errorf("payment %s references unknown tariff %s", p.Payment.ID, p.Payment.TariffID)
		}
		views = append(views, AdminSubscriptionPaymentView{
			Payment:   p.Payment,
			Tariff:    t,
			UserPhone: p.UserPhone,
		})
	}

	return views, total, nil
}
```

**Step 2: Define admin view DTO in `dto.go`**

```go
// AdminSubscriptionPaymentView is a subscription payment with tariff and user phone for admin view.
type AdminSubscriptionPaymentView struct {
	Payment   domain.SubscriptionPayment
	Tariff    domain.Tariff
	UserPhone string
}
```

**Step 3: Build**

Run:

```bash
cd apps/backend && go build ./internal/billing/...
```

Expected: OK.

---

### Task 4: Update OpenAPI contract

**Files:**
- Modify: `apps/backend/api/openapi/openapi.yaml`

**Step 1: Add path under admin section**

After `POST /admin/subscription/payments/{paymentId}/refund`, add:

```yaml
  /admin/subscription/payments:
    get:
      operationId: listAdminSubscriptionPayments
      security:
        - sessionCookie: []
      parameters:
        - name: status
          in: query
          required: false
          schema:
            type: string
            enum: [pending, succeeded, failed, refunded, partial_refunded]
        - name: limit
          in: query
          required: false
          schema:
            type: integer
            default: 20
            minimum: 1
            maximum: 100
        - name: offset
          in: query
          required: false
          schema:
            type: integer
            default: 0
            minimum: 0
      responses:
        '200':
          description: Admin subscription payments list
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/AdminSubscriptionPaymentsResponse'
        '400':
          $ref: '#/components/responses/BadRequest'
        '401':
          $ref: '#/components/responses/Unauthorized'
        '403':
          $ref: '#/components/responses/Forbidden'
        '500':
          $ref: '#/components/responses/InternalServerError'
```

**Step 2: Add schemas**

After `SubscriptionPaymentsResponse`, add:

```yaml
    AdminSubscriptionPayment:
      type: object
      required: [id, tariff, period, amountKopecks, status, provider, userId, userPhone, createdAt]
      properties:
        id:
          type: string
          format: uuid
        tariff:
          $ref: '#/components/schemas/Tariff'
        period:
          type: string
          enum: [month, year]
        amountKopecks:
          type: integer
        status:
          type: string
          enum: [pending, succeeded, failed, refunded, partial_refunded]
        provider:
          type: string
        userId:
          type: string
          format: uuid
        userPhone:
          type: string
        createdAt:
          type: string
          format: date-time

    AdminSubscriptionPaymentsResponse:
      type: object
      required: [items, total]
      properties:
        items:
          type: array
          items:
            $ref: '#/components/schemas/AdminSubscriptionPayment'
        total:
          type: integer
```

**Step 3: Regenerate OpenAPI code**

Run:

```bash
cd apps/backend
go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.0 -config api/openapi/oapi-codegen.yaml api/openapi/openapi.yaml
```

Expected: `internal/platform/openapi/generated.gen.go` now contains `ListAdminSubscriptionPayments` and related types.

---

### Task 5: Implement HTTP handler and route wiring

**Files:**
- Modify: `apps/backend/internal/platform/httpapi/subscription_handlers.go`
- Modify: `apps/backend/internal/platform/httpapi/server.go`

**Step 1: Add presenter helper**

In `subscription_handlers.go`, add:

```go
func adminSubscriptionPaymentResponse(view billingapp.AdminSubscriptionPaymentView) openapi.AdminSubscriptionPayment {
	p := view.Payment
	return openapi.AdminSubscriptionPayment{
		Id:            p.ID,
		Tariff:        tariffResponse(view.Tariff),
		Period:        openapi.AdminSubscriptionPaymentPeriod(p.Period),
		AmountKopecks: int(p.AmountKopecks),
		Status:        openapi.AdminSubscriptionPaymentStatus(p.Status),
		Provider:      string(p.Provider),
		UserId:        p.UserID,
		UserPhone:     view.UserPhone,
		CreatedAt:     p.CreatedAt,
	}
}
```

**Step 2: Add handler method**

```go
// ListAdminSubscriptionPayments implements GET /admin/subscription/payments.
func (h *SubscriptionHandlers) ListAdminSubscriptionPayments(w http.ResponseWriter, r *http.Request, params openapi.ListAdminSubscriptionPaymentsParams) {
	filters := billingapp.ListAllPaymentsFilters{
		Limit:  20,
		Offset: 0,
	}
	if params.Limit != nil {
		filters.Limit = *params.Limit
	}
	if params.Offset != nil {
		filters.Offset = *params.Offset
	}
	if params.Status != nil {
		filters.Status = string(*params.Status)
	}

	views, total, err := h.billing.ListAllPayments(r.Context(), filters)
	if err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	items := make([]openapi.AdminSubscriptionPayment, 0, len(views))
	for _, v := range views {
		items = append(items, adminSubscriptionPaymentResponse(v))
	}
	writeJSON(r.Context(), w, http.StatusOK, openapi.AdminSubscriptionPaymentsResponse{
		Items: items,
		Total: int(total),
	})
}
```

**Step 3: Wire admin route in `server.go`**

After the refund route override, add:

```go
	r.With(AdminOnlyMiddleware).Get("/admin/subscription/payments", wrapper.ListAdminSubscriptionPayments)
```

**Step 4: Build**

Run:

```bash
cd apps/backend && go build ./...
```

Expected: OK.

---

### Task 6: Add tests

**Files:**
- Modify: `apps/backend/internal/billing/application/service_test.go`
- Modify: `apps/backend/internal/platform/httpapi/subscription_handlers_test.go`

**Step 1: Service test**

Add a test that stubs `ListAll` returning two payments and asserts the service returns views with correct totals and respects limit/default.

```go
func TestBillingService_ListAllPayments(t *testing.T) {
	// Arrange: create service with a stub repository that returns two payments.
	// Act: call ListAllPayments with no filters.
	// Assert: 2 items, total 2, ordered by created_at DESC.
}
```

**Step 2: HTTP test**

Add a test that runs the full HTTP server with an admin user in context and asserts:

```go
func TestListAdminSubscriptionPayments(t *testing.T) {
	// admin -> 200
	// non-admin -> 403
	// anonymous -> 401
	// invalid status -> 400
}
```

**Step 3: Run tests**

```bash
cd apps/backend && go test ./internal/billing/application ./internal/platform/httpapi -run 'ListAllPayments|ListAdminSubscriptionPayments' -v
```

Expected: PASS.

---

### Task 7: Add Bruno request

**Files:**
- Create: `tools/bruno/arenda-api/admin/subscription/list payments.bru`

```
meta {
  name: List Payments (admin)
  type: http
  seq: 3
}

get {
  url: {{baseUrl}}/admin/subscription/payments?status={{status}}&limit={{limit}}&offset={{offset}}
  body: none
  auth: none
}

script:pre-request {
  const sessionId = bru.getEnvVar("admin_session_id") || bru.getVar("admin_session_id");
  const cookieName = bru.getEnvVar("adminCookieName") || bru.getVar("adminCookieName") || "session_id";
  if (sessionId) {
    req.setHeader("Cookie", `${cookieName}=${sessionId}`);
  }
}

tests {
  test("should return 200", function() {
    expect(res.status).to.equal(200);
  });

  test("should have items and total", function() {
    expect(res.body.items).to.be.an("array");
    expect(res.body.total).to.be.a("number");
  });
}
```

Add variables to `tools/bruno/arenda-api/environments/Local.bru`:

```text
status:
limit: 20
offset: 0
```

---

### Task 8: Final verification

Run:

```bash
cd apps/backend
go build ./...
go vet ./...
go test ./...
```

And from repo root:

```bash
make backend-lint
```

Expected: all green, 0 lint issues.
