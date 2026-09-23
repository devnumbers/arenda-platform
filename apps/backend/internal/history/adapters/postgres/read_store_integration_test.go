//go:build integration

package postgres_test

// The integration suite of the journal's reading side (карта #704, тикет
// #708, ADR 0061 §7): the SQL visibility (actor_can_read_history, 000137) —
// owner, active members (viewer included), suspended and revoked excluded,
// the archive visible to the owner only — the bidirectional keyset walk
// (before/after, no duplicates, no drops), the filters, the routed search
// through the service seam and the filter-sheet options (current members ∪
// historical actors; objects with the photo avatar).

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	accesspg "github.com/nambers/arenda-planform/apps/backend/internal/access/adapters/postgres"
	accessapp "github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	historypg "github.com/nambers/arenda-planform/apps/backend/internal/history/adapters/postgres"
	historyapp "github.com/nambers/arenda-planform/apps/backend/internal/history/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/testdb"
)

// kindPayment is the wire's payment kind — the filters test's constant.
const kindPayment = "payment"

// newReadStack builds the service over the real store and the real
// membership policy — the same wiring as the composition root.
func newReadStack(pool *pgxpool.Pool) *historyapp.HistoryReadService {
	policy := accessapp.NewMembershipPolicy(accesspg.NewOwnerResolver(pool), accesspg.NewMembershipRepository(pool))
	return historyapp.NewHistoryReadService(historypg.NewReadStore(pool), policy)
}

// seedEntry writes one journal row through the recorder — the actor
// name/email snapshots resolve from the users table exactly as in
// production — then pins created_at to an explicit instant: the recorder
// stamps Now(), and the keyset/period tests need a deterministic order.
func seedEntry(t *testing.T, pool *pgxpool.Pool, actorID, propertyID uuid.UUID, at time.Time,
	kind domain.Kind, action domain.Action, base domain.BaseAction, text string,
) {
	t.Helper()
	entry := domain.Entry{
		PropertyID: propertyID,
		ActorID:    &actorID,
		ActorRole:  domain.ActorRoleOwner,
		Kind:       kind,
		Action:     action,
		BaseAction: base,
		Segments:   domain.Segments{{Text: text}},
		Context:    map[string]any{},
		CreatedAt:  at,
	}
	if err := newRecorder(pool).Record(context.Background(), entry); err != nil {
		t.Fatalf("seed entry: %v", err)
	}
	// Record присваивает ID копии Entry (передача по значению — до
	// вызывающего он не доезжает): свежевставленная строка — последняя по
	// (created_at, id) своего (property, actor), вычитываем её id и пиним
	// созданное время.
	var rowID uuid.UUID
	if err := pool.QueryRow(t.Context(),
		`SELECT id FROM action_journal WHERE property_id = $1 AND actor_id = $2
		 ORDER BY created_at DESC, id DESC LIMIT 1`,
		propertyID, actorID,
	).Scan(&rowID); err != nil {
		t.Fatalf("locate seeded entry: %v", err)
	}
	if _, err := pool.Exec(t.Context(),
		`UPDATE action_journal SET created_at = $1 WHERE id = $2`, at, rowID,
	); err != nil {
		t.Fatalf("pin created_at: %v", err)
	}
}

// suspendMembership flips the membership's status (the slot coordinator's
// raw effect, migration 000095).
func suspendMembership(t *testing.T, pool *pgxpool.Pool, propertyID, userID uuid.UUID) {
	t.Helper()
	if _, err := pool.Exec(t.Context(),
		`UPDATE property_members SET status = 'suspended', suspended_at = now()
		 WHERE property_id = $1 AND user_id = $2`, propertyID, userID,
	); err != nil {
		t.Fatalf("suspend membership: %v", err)
	}
}

func TestReadStore_FeedVisibility(t *testing.T) {
	t.Parallel()
	pool := testdb.Setup(t)
	owner := seedUser(t, pool, "Иван", "Иванов", "+79990000001", "ivan@example.com")
	member := seedUser(t, pool, "Пётр", "Петров", "+79990000002", "petr@example.com")
	viewer := seedUser(t, pool, "Анна", "Сидорова", "+79990000003", "anna@example.com")
	stranger := seedUser(t, pool, "Зина", "Злобина", "+79990000004", "zina@example.com")

	liveProperty := seedProperty(t, pool, owner)
	archivedProperty := seedProperty(t, pool, owner)
	if _, err := pool.Exec(t.Context(),
		`UPDATE properties SET status = 'archived' WHERE id = $1`, archivedProperty,
	); err != nil {
		t.Fatalf("archive property: %v", err)
	}

	seedMembership(t, pool, liveProperty, member)
	seedMembership(t, pool, liveProperty, viewer)

	seedEntry(t, pool, owner, liveProperty, time.Unix(100, 0), domain.KindOperation, domain.ActionOperationPaid,
		domain.BaseCompleted, "Операция оплачена")
	seedEntry(t, pool, owner, archivedProperty, time.Unix(90, 0), domain.KindOperation, domain.ActionOperationPaid,
		domain.BaseCompleted, "Операция оплачена")

	svc := newReadStack(pool)
	ctx := context.Background()

	// Владелец видит оба объекта — история переживает архив.
	page, err := svc.Feed(ctx, owner, historyapp.FeedQuery{})
	if err != nil {
		t.Fatalf("owner feed: %v", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("owner feed: want 2 rows (archive included), got %d", len(page.Items))
	}

	// Активный участник читает ленту живого объекта.
	page, err = svc.Feed(ctx, viewer, historyapp.FeedQuery{})
	if err != nil {
		t.Fatalf("viewer feed: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].PropertyID != liveProperty {
		t.Fatalf("viewer feed: want the live property's row only, got %+v", page.Items)
	}

	// Приостановленный участник ленту не видит вовсе («suspended — недоступно»).
	suspendMembership(t, pool, liveProperty, member)
	page, err = svc.Feed(ctx, member, historyapp.FeedQuery{})
	if err != nil {
		t.Fatalf("suspended feed: %v", err)
	}
	if len(page.Items) != 0 {
		t.Fatalf("suspended member: want no rows, got %d", len(page.Items))
	}

	// Чужой — пусто: существование строк не раскрывается.
	page, err = svc.Feed(ctx, stranger, historyapp.FeedQuery{})
	if err != nil {
		t.Fatalf("stranger feed: %v", err)
	}
	if len(page.Items) != 0 {
		t.Fatalf("stranger: want no rows, got %d", len(page.Items))
	}
}

func TestReadStore_InvisiblePropertyIDIsPrivacy404(t *testing.T) {
	t.Parallel()
	pool := testdb.Setup(t)
	owner := seedUser(t, pool, "Иван", "Иванов", "+79990000001", "ivan@example.com")
	member := seedUser(t, pool, "Пётр", "Петров", "+79990000002", "petr@example.com")
	stranger := seedUser(t, pool, "Зина", "Злобина", "+79990000003", "zina@example.com")
	property := seedProperty(t, pool, owner)
	seedMembership(t, pool, property, member)
	seedEntry(t, pool, owner, property, time.Unix(100, 0), domain.KindOperation, domain.ActionOperationPaid,
		domain.BaseCompleted, "Операция оплачена")

	svc := newReadStack(pool)
	unknown := uuid.Must(uuid.NewV7())

	// Чужой id и неизвестный — одинаково privacy-404; смесь живого и чужого
	// не палит, какой именно чужой. Читателю с доступом тот же id проходит.
	for _, actor := range []uuid.UUID{stranger, member} {
		ids := []uuid.UUID{unknown}
		if actor == member {
			ids = []uuid.UUID{unknown, property}
		}
		if _, err := svc.Feed(context.Background(), actor, historyapp.FeedQuery{PropertyIDs: ids}); !errors.Is(err, historyapp.ErrNotFound) {
			t.Fatalf("privacy 404: want ErrNotFound, got %v", err)
		}
		if _, err := svc.Filters(context.Background(), actor, ids); !errors.Is(err, historyapp.ErrNotFound) {
			t.Fatalf("privacy 404 on filters: want ErrNotFound, got %v", err)
		}
	}
	if page, err := svc.Feed(context.Background(), owner, historyapp.FeedQuery{PropertyIDs: []uuid.UUID{property}}); err != nil ||
		len(page.Items) != 1 {
		t.Fatalf("owner scoped feed: %v / %d rows", err, len(page.Items))
		t.Fatalf("owner scoped feed: %v / %d rows", err, len(page.Items))
	}
}

func TestReadStore_DownwardKeysetWalk(t *testing.T) {
	t.Parallel()
	pool := testdb.Setup(t)
	owner := seedUser(t, pool, "Иван", "Иванов", "+79990000001", "ivan@example.com")
	property := seedProperty(t, pool, owner)

	// 12 строк с шагом в секунду — детерминированный порядок (created_at, id).
	total := 12
	for i := range total {
		seedEntry(t, pool, owner, property, time.Unix(int64(1000+i), 0),
			domain.KindOperation, domain.ActionOperationPaid, domain.BaseCompleted, fmt.Sprintf("Строка %d", i))
	}
	svc := newReadStack(pool)
	ctx := context.Background()

	const pageSize = 5
	seen := map[uuid.UUID]bool{}
	tail := historyapp.FeedPage{}
	page, err := svc.Feed(ctx, owner, historyapp.FeedQuery{Limit: pageSize})
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	for pages := 1; ; pages++ {
		if pages > 5 {
			t.Fatal("the walk did not end within 5 pages")
		}
		for _, item := range page.Items {
			if seen[item.ID] {
				t.Fatalf("duplicate row %s across the downward walk", item.ID)
			}
			seen[item.ID] = true
		}
		if len(page.Items) < pageSize {
			tail = page
			break
		}
		if page.NextCursor == "" {
			t.Fatal("a full page must carry next_cursor")
		}
		page, err = svc.Feed(ctx, owner, historyapp.FeedQuery{Limit: pageSize, BeforeCursor: page.NextCursor})
		if err != nil {
			t.Fatalf("next page: %v", err)
		}
	}
	if len(seen) != total {
		t.Fatalf("downward walk: want all %d rows, got %d", total, len(seen))
	}
	if want := total - total/pageSize*pageSize; len(tail.Items) != want {
		t.Fatalf("tail page size: want %d, got %d", want, len(tail.Items))
	}
}

// walkToTail walks the feed down to the last page, collecting the seen ids —
// the shared prep of the bidirectional walk tests.
func walkToTail(
	t *testing.T, svc *historyapp.HistoryReadService, owner uuid.UUID, pageSize int,
) (seen map[uuid.UUID]bool, tail historyapp.FeedPage) {
	t.Helper()
	ctx := context.Background()
	seen = map[uuid.UUID]bool{}
	page, err := svc.Feed(ctx, owner, historyapp.FeedQuery{Limit: pageSize})
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	for len(page.Items) == pageSize {
		for _, item := range page.Items {
			seen[item.ID] = true
		}
		if page.NextCursor == "" {
			t.Fatal("a full page must carry next_cursor")
		}
		page, err = svc.Feed(ctx, owner, historyapp.FeedQuery{Limit: pageSize, BeforeCursor: page.NextCursor})
		if err != nil {
			t.Fatalf("next page: %v", err)
		}
	}
	for _, item := range page.Items {
		seen[item.ID] = true
	}
	return seen, page
}

func TestReadStore_UpwardKeysetWalkAndPrepend(t *testing.T) {
	t.Parallel()
	pool := testdb.Setup(t)
	owner := seedUser(t, pool, "Иван", "Иванов", "+79990000001", "ivan@example.com")
	property := seedProperty(t, pool, owner)

	total := 12
	for i := range total {
		seedEntry(t, pool, owner, property, time.Unix(int64(1000+i), 0),
			domain.KindOperation, domain.ActionOperationPaid, domain.BaseCompleted, fmt.Sprintf("Строка %d", i))
	}
	svc := newReadStack(pool)
	ctx := context.Background()

	const pageSize = 5
	// Вниз до хвоста — стартовая точка восходящего обхода.
	seen, tail := walkToTail(t, svc, owner, pageSize)

	// Вверх по времени: страницы after_cursor возвращают строго более новые
	// строки; объединение вниз-вверх покрывает ровно все строки без потерь.
	got := map[uuid.UUID]bool{}
	after := tail.PrevCursor
	for after != "" {
		page, err := svc.Feed(ctx, owner, historyapp.FeedQuery{Limit: pageSize, AfterCursor: after})
		if err != nil {
			t.Fatalf("newer page: %v", err)
		}
		if len(page.Items) == 0 {
			break
		}
		for i := 1; i < len(page.Items); i++ {
			if page.Items[i].CreatedAt.After(page.Items[i-1].CreatedAt) {
				t.Fatal("an after-page must keep the (created_at, id) DESC order")
			}
		}
		for _, item := range page.Items {
			got[item.ID] = true
		}
		if len(page.Items) < pageSize {
			break
		}
		after = page.PrevCursor
	}
	for id := range seen {
		got[id] = true
	}
	if len(got) != total {
		t.Fatalf("upward walk union: want %d rows, got %d", total, len(got))
	}

	first, firstErr := svc.Feed(ctx, owner, historyapp.FeedQuery{Limit: pageSize})
	if firstErr != nil {
		t.Fatalf("first page for cursors: %v", firstErr)
	}
	if first.NextCursor == "" || first.PrevCursor == "" {
		t.Fatal("a full first page must carry both cursors")
	}
}

func TestReadStore_PrependNewArrival(t *testing.T) {
	t.Parallel()
	pool := testdb.Setup(t)
	owner := seedUser(t, pool, "Иван", "Иванов", "+79990000001", "ivan@example.com")
	property := seedProperty(t, pool, owner)
	for i := range 6 {
		seedEntry(t, pool, owner, property, time.Unix(int64(1000+i), 0),
			domain.KindOperation, domain.ActionOperationPaid, domain.BaseCompleted, fmt.Sprintf("Строка %d", i))
	}
	svc := newReadStack(pool)
	ctx := context.Background()

	// Страница, снятая до прихода, несёт ключ первой (самой свежей) строки;
	// prepend по after_cursor возвращает ровно свежую строку — без дублей
	// и потерь.
	first, err := svc.Feed(ctx, owner, historyapp.FeedQuery{Limit: 5})
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	seedEntry(t, pool, owner, property, time.Unix(2000, 0),
		domain.KindPayment, domain.ActionPaymentCreated, domain.BaseAdded, "Свежая строка")
	page, err := svc.Feed(ctx, owner, historyapp.FeedQuery{Limit: 5, AfterCursor: first.PrevCursor})
	if err != nil {
		t.Fatalf("prepend page: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Segments[0].Text != "Свежая строка" {
		t.Fatalf("prepend: want exactly the fresh row, got %+v", page.Items)
	}
}

func TestReadStore_Filters(t *testing.T) {
	t.Parallel()
	pool := testdb.Setup(t)
	owner := seedUser(t, pool, "Иван", "Иванов", "+79990000001", "ivan@example.com")
	member := seedUser(t, pool, "Пётр", "Петров", "+79990000002", "petr@example.com")
	propertyA := seedProperty(t, pool, owner)
	propertyB := seedProperty(t, pool, owner)
	seedMembership(t, pool, propertyA, member)

	base := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	seedEntry(t, pool, owner, propertyA, base, domain.KindPayment, domain.ActionPaymentCreated, domain.BaseAdded, "Платёж создан")
	seedEntry(t, pool, owner, propertyA, base.Add(time.Second), domain.KindOperation, domain.ActionOperationPaid,
		domain.BaseCompleted, "Операция оплачена")
	seedEntry(t, pool, member, propertyA, base.Add(2*time.Second), domain.KindTask, domain.ActionTaskCompleted,
		domain.BaseCompleted, "Задача выполнена")
	seedEntry(t, pool, owner, propertyB, base.Add(3*time.Second), domain.KindPayment, domain.ActionPaymentCreated,
		domain.BaseAdded, "Платёж создан")

	svc := newReadStack(pool)
	ctx := context.Background()
	feed := func(q historyapp.FeedQuery) []domain.FeedEntry {
		t.Helper()
		page, err := svc.Feed(ctx, owner, q)
		if err != nil {
			t.Fatalf("feed: %v", err)
		}
		return page.Items
	}

	if got := len(feed(historyapp.FeedQuery{Kinds: []string{kindPayment}})); got != 2 {
		t.Fatalf("kinds=payment: want 2 rows, got %d", got)
	}
	if got := len(feed(historyapp.FeedQuery{BaseActions: []string{"completed"}})); got != 2 {
		t.Fatalf("base=completed: want 2 rows, got %d", got)
	}
	if got := len(feed(historyapp.FeedQuery{ActorIDs: []uuid.UUID{member}})); got != 1 {
		t.Fatalf("actor=member: want 1 row, got %d", got)
	}
	if got := len(feed(historyapp.FeedQuery{PropertyIDs: []uuid.UUID{propertyA}})); got != 3 {
		t.Fatalf("property=A: want 3 rows, got %d", got)
	}
	if got := len(feed(historyapp.FeedQuery{Kinds: []string{kindPayment}, PropertyIDs: []uuid.UUID{propertyA}})); got != 1 {
		t.Fatalf("kinds+property: want 1 row, got %d", got)
	}
	// Период: границы — календарные дни; верхняя исключающая (дата + 24ч).
	day := func(y int, m time.Month, d int) *time.Time {
		t0 := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
		return &t0
	}
	if got := len(feed(historyapp.FeedQuery{DateFrom: day(2026, 9, 20), DateTo: day(2026, 9, 21)})); got != 4 {
		t.Fatalf("period=Sep 20: want 4 rows, got %d", got)
	}
	if got := len(feed(historyapp.FeedQuery{DateFrom: day(2026, 9, 21)})); got != 0 {
		t.Fatalf("period=Sep 21+: want 0 rows, got %d", got)
	}
}

func TestReadStore_SearchRouting(t *testing.T) {
	t.Parallel()
	pool := testdb.Setup(t)
	owner := seedUser(t, pool, "Иван", "Иванов", "+79990000001", "ivan@example.com")
	member := seedUser(t, pool, "Пётр", "Петров", "+79990000002", "petr.petrov@example.com")
	property := seedProperty(t, pool, owner)
	seedMembership(t, pool, property, member)

	base := time.Unix(1000, 0)
	// «Изменено» — словоформа: находит FTS-нога маршрутизации.
	seedEntry(t, pool, owner, property, base, domain.KindProperty, domain.ActionPropertyRenamed,
		domain.BaseChanged, "Название объекта изменено")
	// Почта и снапшот имени участника — территория trgm-ноги.
	seedEntry(t, pool, member, property, base.Add(time.Second), domain.KindContact, domain.ActionContactCreated,
		domain.BaseAdded, "Контакт создан: Анна Ленина")

	svc := newReadStack(pool)
	ctx := context.Background()
	search := func(actor uuid.UUID, q string) int {
		t.Helper()
		page, err := svc.Feed(ctx, actor, historyapp.FeedQuery{Query: q})
		if err != nil {
			t.Fatalf("search %q: %v", q, err)
		}
		return len(page.Items)
	}

	if got := search(owner, "изменено"); got != 1 {
		t.Fatalf("fts morphology («изменено»): want 1 row, got %d", got)
	}
	if got := search(owner, "Название объекта изменено"); got != 1 {
		t.Fatalf("fts multiword: want 1 row, got %d", got)
	}
	if got := search(owner, "petr.petrov"); got != 1 {
		t.Fatalf("trgm email local part: want 1 row, got %d", got)
	}
	// Обе почты кончаются на @example — trgm-нога находит обе строки.
	if got := search(owner, "@example"); got != 2 {
		t.Fatalf("trgm email fragment: want 2 rows, got %d", got)
	}
	if got := search(owner, "qqqqzzzz"); got != 0 {
		t.Fatalf("garbage query: want 0 rows, got %d", got)
	}
	// Поиск уважает скоуп: зритель находит строку владельца по его имени,
	// свою — по своей почте.
	if got := search(member, "Иванов"); got != 1 {
		t.Fatalf("member search by owner name: want 1 row, got %d", got)
	}
	if got := search(member, "Петров"); got != 1 {
		t.Fatalf("member search by own name: want 1 row, got %d", got)
	}
}

func TestReadStore_FilterOptionsParticipants(t *testing.T) {
	t.Parallel()
	pool := testdb.Setup(t)
	owner := seedUser(t, pool, "Иван", "Иванов", "+79990000001", "ivan@example.com")
	member := seedUser(t, pool, "Пётр", "Петров", "+79990000002", "petr@example.com")
	viewer := seedUser(t, pool, "Анна", "Сидорова", "+79990000003", "anna@example.com")
	stranger := seedUser(t, pool, "Зина", "Злобина", "+79990000004", "zina@example.com")

	property := seedProperty(t, pool, owner)
	otherProperty := seedProperty(t, pool, stranger)
	seedMembership(t, pool, property, member)
	seedMembership(t, pool, property, viewer)

	base := time.Unix(1000, 0)
	// Отозванный актёр: вышел из объекта — записи остались,
	// фильтруемым остался.
	exited := seedUser(t, pool, "Олег", "Ушёл", "+79990000005", "oleg@example.com")
	seedMembership(t, pool, property, exited)
	seedEntry(t, pool, exited, property, base,
		domain.KindRental, domain.ActionRentalCreated, domain.BaseAdded, "Аренда создана")
	if _, err := pool.Exec(t.Context(),
		`DELETE FROM property_members WHERE property_id = $1 AND user_id = $2`, property, exited,
	); err != nil {
		t.Fatalf("revoke membership: %v", err)
	}
	// Обезличенный актёр: пользователь удалён (actor_id SET NULL) — в
	// опциях его нет, строка остаётся.
	ghost := seedUser(t, pool, "Лев", "Призрак", "+79990000006", "ghost@example.com")
	seedEntry(t, pool, ghost, property, base.Add(time.Second),
		domain.KindTask, domain.ActionTaskCompleted, domain.BaseCompleted, "Задача выполнена")
	if _, err := pool.Exec(t.Context(), `DELETE FROM users WHERE id = $1`, ghost); err != nil {
		t.Fatalf("delete ghost user: %v", err)
	}
	seedEntry(t, pool, owner, property, base.Add(2*time.Second),
		domain.KindPayment, domain.ActionPaymentCreated, domain.BaseAdded, "Платёж создан")

	// Приостановленный участник остаётся в списке: suspend не вытирает его
	// из состава участников.
	suspendMembership(t, pool, property, viewer)

	svc := newReadStack(pool)
	opts, err := svc.Filters(context.Background(), member, nil)
	if err != nil {
		t.Fatalf("filters: %v", err)
	}
	// Зритель-читатель видит: владельца, участника-себя, приостановленную
	// Анну и отозванного Олега; призрак удалён — не фильтруем; Зина на
	// чужом объекте — нет.
	names := map[string]bool{}
	for _, p := range opts.Participants {
		names[p.Name] = true
	}
	for _, want := range []string{"Иван Иванов", "Пётр Петров", "Анна Сидорова", "Олег Ушёл"} {
		if !names[want] {
			t.Errorf("participants: want %q in %+v", want, names)
		}
	}
	for _, banned := range []string{"Лев Призрак", "Зина Злобина"} {
		if names[banned] {
			t.Errorf("participants: %q must not be listed", banned)
		}
	}

	// Скоуп по property_ids сужает участников: журнал другого объекта
	// не подмешивается (зовёт владелец этого объекта).
	scopedOpts, err := svc.Filters(context.Background(), stranger, []uuid.UUID{otherProperty})
	if err != nil {
		t.Fatalf("scoped filters: %v", err)
	}
	if len(scopedOpts.Participants) != 1 || scopedOpts.Participants[0].ID != stranger {
		t.Fatalf("scoped participants: want only Зина, got %+v", scopedOpts.Participants)
	}
}

// Владелец-замок считается по любому объекту скоупа, а не по роли на
// конкретном: приглашённый, владеющий вторым объектом той же области
// чтения, помечен is_owner; приглашённый без своих объектов — нет.
// Замок у того, кто пригласил (#711, макет 2067-163528). First_name —
// имя без фамилии; у пользователя без имени — пусто (name — маскированный
// телефон).
func TestReadStore_FilterOptionsParticipantOwnerFlag(t *testing.T) {
	t.Parallel()
	pool := testdb.Setup(t)
	const (
		namelessPhone    = "+79990000009"
		ownerDisplayName = "Иван Иванов"
	)
	owner := seedUser(t, pool, "Иван", "Иванов", "+79990000001", "ivan@example.com")
	member := seedUser(t, pool, "Пётр", "Петров", "+79990000002", "petr@example.com")
	invited := seedUser(t, pool, "Анна", "Сидорова", "+79990000003", "anna@example.com")
	nameless := seedUser(t, pool, "", "", namelessPhone, "nameless@example.com")

	property := seedProperty(t, pool, owner)
	ownProperty := seedProperty(t, pool, member) // Пётр владеет вторым объектом области.
	seedMembership(t, pool, property, member)
	seedMembership(t, pool, property, invited)
	seedMembership(t, pool, property, nameless)
	seedMembership(t, pool, ownProperty, owner) // Иван читает оба объекта.

	svc := newReadStack(pool)
	opts, err := svc.Filters(context.Background(), owner, nil)
	if err != nil {
		t.Fatalf("filters: %v", err)
	}
	if len(opts.Participants) != 4 {
		t.Fatalf("participants: want 4, got %+v", opts.Participants)
	}
	byID := map[uuid.UUID]domain.FilterParticipant{}
	for _, p := range opts.Participants {
		byID[p.ID] = p
	}
	// Замок по любому объекту скоупа: Пётр на первом объекте участник, но
	// владелец второго. First_name — имя без фамилии; у пользователя без
	// имени — пусто и name замаскирован.
	tests := []struct {
		name      string
		p         domain.FilterParticipant
		wantOwner bool
		wantFirst string
		wantName  string
	}{
		{"owner", byID[owner], true, "Иван", ownerDisplayName},
		{"member owning the second object", byID[member], true, "Пётр", "Пётр Петров"},
		{"invited without own objects", byID[invited], false, "Анна", "Анна Сидорова"},
		{"nameless", byID[nameless], false, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.p.IsOwner != tt.wantOwner {
				t.Errorf("is_owner: want %v, got %+v", tt.wantOwner, tt.p)
			}
			if tt.p.FirstName != tt.wantFirst {
				t.Errorf("first_name: want %q, got %+v", tt.wantFirst, tt.p)
			}
			if tt.wantName != "" && tt.p.Name != tt.wantName {
				t.Errorf("name: want %q, got %+v", tt.wantName, tt.p)
			}
		})
	}
	if name := byID[nameless].Name; name == "" || name == namelessPhone {
		t.Errorf("nameless name: want masked phone, got %q", name)
	}

	// Скоуп по property_ids: единственный участник — владелец суженной
	// области, замок и first_name доходят и в суженном запросе.
	stranger := seedUser(t, pool, "Зина", "Злобина", "+79990000004", "zina@example.com")
	foreignProperty := seedProperty(t, pool, stranger)
	scopedOpts, err := svc.Filters(context.Background(), stranger, []uuid.UUID{foreignProperty})
	if err != nil {
		t.Fatalf("scoped filters: %v", err)
	}
	if len(scopedOpts.Participants) != 1 {
		t.Fatalf("scoped participants: want only Зина, got %+v", scopedOpts.Participants)
	}
	if p := scopedOpts.Participants[0]; !p.IsOwner || p.FirstName != "Зина" {
		t.Fatalf("scoped participants: want is_owner + first name Зина, got %+v", p)
	}
}

func TestReadStore_FilterOptionsObjects(t *testing.T) {
	t.Parallel()
	pool := testdb.Setup(t)
	owner := seedUser(t, pool, "Иван", "Иванов", "+79990000001", "ivan@example.com")
	stranger := seedUser(t, pool, "Зина", "Злобина", "+79990000004", "zina@example.com")

	property := seedProperty(t, pool, owner)
	seedProperty(t, pool, stranger)

	// Фото-аватар: два фото, берётся первое по времени.
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO property_photos (id, property_id, url, created_at)
		 VALUES ($1, $2, 'http://x/second.jpg', now() + interval '2 seconds')`,
		uuid.Must(uuid.NewV7()), property,
	); err != nil {
		t.Fatalf("seed photo 1: %v", err)
	}
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO property_photos (id, property_id, url, created_at)
		 VALUES ($1, $2, 'http://x/first.jpg', now())`,
		uuid.Must(uuid.NewV7()), property,
	); err != nil {
		t.Fatalf("seed photo 2: %v", err)
	}

	svc := newReadStack(pool)
	opts, err := svc.Filters(context.Background(), owner, nil)
	if err != nil {
		t.Fatalf("filters: %v", err)
	}
	// Чужой объект в опциях не попадает.
	if len(opts.Objects) != 1 || opts.Objects[0].ID != property {
		t.Fatalf("objects: want only the visible one, got %+v", opts.Objects)
	}
	if opts.Objects[0].PhotoURL != "http://x/first.jpg" {
		t.Fatalf("photo avatar: want the oldest photo, got %q", opts.Objects[0].PhotoURL)
	}
}

func TestReadStore_ArchivedScopeForOptions(t *testing.T) {
	t.Parallel()
	pool := testdb.Setup(t)
	owner := seedUser(t, pool, "Иван", "Иванов", "+79990000001", "ivan@example.com")
	member := seedUser(t, pool, "Пётр", "Петров", "+79990000002", "petr@example.com")
	property := seedProperty(t, pool, owner)
	seedMembership(t, pool, property, member)
	seedEntry(t, pool, owner, property, time.Unix(100, 0), domain.KindOperation, domain.ActionOperationPaid,
		domain.BaseCompleted, "Операция оплачена")
	if _, err := pool.Exec(t.Context(),
		`UPDATE properties SET status = 'archived' WHERE id = $1`, property,
	); err != nil {
		t.Fatalf("archive property: %v", err)
	}

	svc := newReadStack(pool)
	ctx := context.Background()

	// Владелец: архивный объект остаётся в опциях вместе с историей.
	opts, err := svc.Filters(ctx, owner, nil)
	if err != nil {
		t.Fatalf("owner filters: %v", err)
	}
	if len(opts.Objects) != 1 {
		t.Fatalf("owner objects: archived stays visible, got %+v", opts.Objects)
	}

	// Участник: архив ничего не добавляет (канон невидимости архива, #163).
	opts, err = svc.Filters(ctx, member, nil)
	if err != nil {
		t.Fatalf("member filters: %v", err)
	}
	if len(opts.Objects) != 0 || len(opts.Participants) != 0 {
		t.Fatalf("member options: archived contributes nothing, got %+v / %+v", opts.Objects, opts.Participants)
	}
}
