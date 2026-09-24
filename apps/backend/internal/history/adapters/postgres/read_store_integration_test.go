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

func TestReadStore_SearchRespectsScope(t *testing.T) {
	t.Parallel()
	pool := testdb.Setup(t)
	owner := seedUser(t, pool, "Иван", "Иванов", "+79990000001", "ivan@example.com")
	member := seedUser(t, pool, "Пётр", "Петров", "+79990000002", "petr.petrov@example.com")
	property := seedProperty(t, pool, owner)
	seedMembership(t, pool, property, member)

	base := time.Unix(1000, 0)
	seedEntry(t, pool, owner, property, base, domain.KindProperty, domain.ActionPropertyRenamed,
		domain.BaseChanged, "Название объекта изменено")
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

	if got := search(owner, "qqqqzzzz"); got != 0 {
		t.Fatalf("garbage query: want 0 rows, got %d", got)
	}
	// Поиск уважает скоуп: зритель находит строку владельца по его имени,
	// свою — по своей почте (семантика предиката — TestReadStore_SearchPredicate).
	if got := search(member, "Иванов"); got != 1 {
		t.Fatalf("member search by owner name: want 1 row, got %d", got)
	}
	if got := search(member, "Петров"); got != 1 {
		t.Fatalf("member search by own name: want 1 row, got %d", got)
	}
}

// newPredicateSearch создаёт замыкание поиска от лица владельца: на вход
// строка, на выходе множество id строк выдачи.
func newPredicateSearch(t *testing.T, svc *historyapp.HistoryReadService, owner uuid.UUID) func(string) map[uuid.UUID]bool {
	t.Helper()
	ctx := context.Background()
	return func(q string) map[uuid.UUID]bool {
		page, err := svc.Feed(ctx, owner, historyapp.FeedQuery{Query: q})
		if err != nil {
			t.Fatalf("search %q: %v", q, err)
		}
		ids := make(map[uuid.UUID]bool, len(page.Items))
		for _, item := range page.Items {
			ids[item.ID] = true
		}
		return ids
	}
}

// searchPredicateCount сверяет выдачу поиска с точным ожиданием строк.
func searchPredicateCount(t *testing.T, search func(string) map[uuid.UUID]bool, q string, want int) {
	t.Helper()
	if got := len(search(q)); got != want {
		t.Fatalf("search %q: want %d rows, got %d", q, want, got)
	}
}

// newRowIDAt создаёт вычитку засеянной строки по зафиксированному created_at.
func newRowIDAt(t *testing.T, pool *pgxpool.Pool, property uuid.UUID) func(time.Time) uuid.UUID {
	t.Helper()
	return func(at time.Time) uuid.UUID {
		var id uuid.UUID
		if err := pool.QueryRow(context.Background(),
			`SELECT id FROM action_journal WHERE property_id = $1 AND created_at = $2`,
			property, at,
		).Scan(&id); err != nil {
			t.Fatalf("locate seeded row at %s: %v", at, err)
		}
		return id
	}
}

// searchPredicatePrefixes проверяет префиксные сценарии: «петр» находит
// «Петрова»; любой ввод семьи «иван/иванов/Иванова» находит обе формы
// фамилии (стеммер асимметричен, и to_tsquery нормализует лексему
// префикса: 'иванов':* живёт как 'ива':* — выручает союз prefix-FTS и
// ILIKE-ног). Имена актёров в searchable («Иван Иванов» в каждой строке
// владельца) точные счётчики ломают — семья фамилии проверяется сабсетом.
func searchPredicatePrefixes(t *testing.T, search func(string) map[uuid.UUID]bool, idsA, idsE uuid.UUID) {
	t.Helper()
	searchPredicateCount(t, search, "петр", 1)
	for _, q := range []string{"иван", "иванов", "ив", "Иванова"} {
		got := search(q)
		if !got[idsA] || !got[idsE] {
			t.Fatalf("prefix %q: want both surname forms found, got %v rows", q, len(got))
		}
	}
}

// searchPredicateKeyset проверяет пустой ввод (фильтра нет — лента целиком)
// и постраничный обход выдачи keyset'ом без дублей и дыр: «счёт» находит
// три строки (две «выставила счёт» и «счёт 100% оплачен»), порция в две.
func searchPredicateKeyset(t *testing.T, svc *historyapp.HistoryReadService, owner uuid.UUID, total int) {
	t.Helper()
	ctx := context.Background()
	for _, q := range []string{"", "   "} {
		page, err := svc.Feed(ctx, owner, historyapp.FeedQuery{Query: q})
		if err != nil {
			t.Fatalf("empty search %q: %v", q, err)
		}
		if len(page.Items) != total {
			t.Fatalf("empty query %q: want %d rows, got %d", q, total, len(page.Items))
		}
	}
	seen := map[uuid.UUID]bool{}
	cursor := ""
	pages := 0
	for {
		page, err := svc.Feed(ctx, owner, historyapp.FeedQuery{Query: "счёт", Limit: 2, BeforeCursor: cursor})
		if err != nil {
			t.Fatalf("search page %d: %v", pages, err)
		}
		for _, item := range page.Items {
			if seen[item.ID] {
				t.Fatalf("search page %d: duplicate row %s", pages, item.ID)
			}
			seen[item.ID] = true
		}
		pages++
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	if len(seen) != 3 || pages != 2 {
		t.Fatalf("search keyset walk: want 3 rows over 2 pages, got %d rows over %d pages", len(seen), pages)
	}
}

// TestReadStore_SearchPredicate гоняет всегда-OR предикат поиска (ресерч
// #839, тикет #842) по сценарию «как в Telegram»: префикс слова, фрагмент
// внутри слова, морфология, почта, обрубки и инъекции — на одном сиде
// («Иванов заплатил», «Петрова выставила счёт», «renewed contract»,
// «счёт 100% оплачен», «Иванова выставила счёт», «ivanov.petr@example.com»).
func TestReadStore_SearchPredicate(t *testing.T) {
	t.Parallel()
	pool := testdb.Setup(t)
	owner := seedUser(t, pool, "Иван", "Иванов", "+79990000001", "ivan@example.com")
	maria := seedUser(t, pool, "Мария", "Петрова", "+79990000002", "maria@example.com")
	property := seedProperty(t, pool, owner)
	seedMembership(t, pool, property, maria)

	base := time.Unix(1000, 0)
	seedEntry(t, pool, owner, property, base, domain.KindPayment, domain.ActionPaymentCreated,
		domain.BaseAdded, "Иванов заплатил")
	seedEntry(t, pool, maria, property, base.Add(time.Second), domain.KindOperation, domain.ActionOperationPaid,
		domain.BaseCompleted, "Петрова выставила счёт")
	seedEntry(t, pool, owner, property, base.Add(2*time.Second), domain.KindProperty, domain.ActionPropertyRenamed,
		domain.BaseChanged, "renewed contract")
	seedEntry(t, pool, owner, property, base.Add(3*time.Second), domain.KindOperation, domain.ActionOperationPaid,
		domain.BaseCompleted, "счёт 100% оплачен")
	seedEntry(t, pool, owner, property, base.Add(4*time.Second), domain.KindOperation, domain.ActionOperationPaid,
		domain.BaseCompleted, "Иванова выставила счёт")
	seedEntry(t, pool, owner, property, base.Add(5*time.Second), domain.KindContact, domain.ActionContactCreated,
		domain.BaseAdded, "ivanov.petr@example.com")

	svc := newReadStack(pool)
	const total = 6
	search := newPredicateSearch(t, svc, owner)
	rowIDAt := newRowIDAt(t, pool, property)
	idsA, idsE := rowIDAt(base), rowIDAt(base.Add(4*time.Second))

	t.Run("префиксы", func(t *testing.T) {
		t.Parallel()
		searchPredicatePrefixes(t, search, idsA, idsE)
	})
	t.Run("морфология и фрагменты", func(t *testing.T) {
		t.Parallel()
		// «заплатила» → «заплатил», «renewing» → «renewed»; «трова» —
		// фрагмент внутри слова, видит только ILIKE-нога.
		searchPredicateCount(t, search, "заплатила", 1)
		searchPredicateCount(t, search, "renewing", 1)
		searchPredicateCount(t, search, "трова", 1)
	})
	t.Run("почта", func(t *testing.T) {
		t.Parallel()
		// Обрубок локальной части, точка внутри локальной части, домен.
		searchPredicateCount(t, search, "maria@", 1)
		searchPredicateCount(t, search, "ivanov.petr", 1)
		searchPredicateCount(t, search, "@example.com", total)
	})
	t.Run("обрубки и цифры", func(t *testing.T) {
		t.Parallel()
		// Хвостовая точка не мешает префиксу, отсутствующие цифры дают
		// пустую выдачу без ошибки; один символ «ё» живёт только в «счёт»
		// (три строки), FTS-нога на стоп-слове молчит — ищет ILIKE-нога.
		searchPredicateCount(t, search, "петр.", 1)
		searchPredicateCount(t, search, "125000", 0)
		searchPredicateCount(t, search, "ё", 3)
	})
	t.Run("многословие", func(t *testing.T) {
		t.Parallel()
		// AND по лексемам; последнее слово едет префиксом (семантика
		// инкрементального ввода — «выставила сч» находит обе «выставила
		// счёт»), стоп-слово снимается до приклейки ':*'.
		searchPredicateCount(t, search, "иванова выставила", 1)
		searchPredicateCount(t, search, "выставила сч", 2)
		searchPredicateCount(t, search, "петр и", 1)
	})
	t.Run("инъекции", func(t *testing.T) {
		t.Parallel()
		// Не роняют запрос и не превращаются в wildcard: '%' и '_' ищутся
		// литерально (escapeLikePattern + ESCAPE '\').
		searchPredicateCount(t, search, "'", 0)
		searchPredicateCount(t, search, "'; --", 0)
		searchPredicateCount(t, search, "_", 0)
		searchPredicateCount(t, search, "0%", 1)
	})
	t.Run("пустое и keyset", func(t *testing.T) {
		t.Parallel()
		searchPredicateKeyset(t, svc, owner, total)
	})
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

// Роль строки шита (#840, макет 2184-94261): максимальный доступ в области
// — владелец → owner, живое членство → его роль, отозванный актёр → роль
// из снимка журнала (actor_role); у человека с членством и журнальным
// снимком разных ролей побеждает более широкая (MIN ранга).
func TestReadStore_FilterOptionsParticipantRoles(t *testing.T) {
	t.Parallel()
	pool := testdb.Setup(t)
	owner := seedUser(t, pool, "Иван", "Иванов", "+79990000001", "ivan@example.com")
	fullAccess := seedUser(t, pool, "Пётр", "Петров", "+79990000002", "petr@example.com")
	viewer := seedUser(t, pool, "Анна", "Сидорова", "+79990000003", "anna@example.com")
	exited := seedUser(t, pool, "Олег", "Ушёл", "+79990000004", "oleg@example.com")
	mixed := seedUser(t, pool, "Мария", "Петрова", "+79990000005", "maria@example.com")

	property := seedProperty(t, pool, owner)
	seedMembership(t, pool, property, fullAccess)
	seedMembership(t, pool, property, exited)
	// Viewer-членство прямым INSERT'ом: хелпер сеет только full_access.
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO property_members (id, property_id, user_id, role, granted_by)
		 VALUES ($1, $2, $3, 'viewer', $3)`,
		uuid.Must(uuid.NewV7()), property, viewer,
	); err != nil {
		t.Fatalf("seed viewer membership: %v", err)
	}
	// Олег вышел из объекта — в опциях остаётся снимком журнала
	// (actor_role full_access).
	exitedEntry := domain.Entry{
		PropertyID: property,
		ActorID:    &exited,
		ActorRole:  domain.ActorRoleFullAccess,
		Kind:       domain.KindRental,
		Action:     domain.ActionRentalCreated,
		BaseAction: domain.BaseAdded,
		Segments:   domain.Segments{{Text: "Аренда создана"}},
		Context:    map[string]any{},
		CreatedAt:  time.Unix(1000, 0),
	}
	if err := newRecorder(pool).Record(context.Background(), exitedEntry); err != nil {
		t.Fatalf("seed exited entry: %v", err)
	}
	if _, err := pool.Exec(t.Context(),
		`DELETE FROM property_members WHERE property_id = $1 AND user_id = $2`, property, exited,
	); err != nil {
		t.Fatalf("revoke membership: %v", err)
	}
	// Мария: viewer-членство на объекте + журнальный снимок full_access —
	// более широкая роль побеждает.
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO property_members (id, property_id, user_id, role, granted_by)
		 VALUES ($1, $2, $3, 'viewer', $3)`,
		uuid.Must(uuid.NewV7()), property, mixed,
	); err != nil {
		t.Fatalf("seed mixed membership: %v", err)
	}
	mixedEntry := domain.Entry{
		PropertyID: property,
		ActorID:    &mixed,
		ActorRole:  domain.ActorRoleFullAccess,
		Kind:       domain.KindTask,
		Action:     domain.ActionTaskCompleted,
		BaseAction: domain.BaseCompleted,
		Segments:   domain.Segments{{Text: "Задача выполнена"}},
		Context:    map[string]any{},
		CreatedAt:  time.Unix(1001, 0),
	}
	if err := newRecorder(pool).Record(context.Background(), mixedEntry); err != nil {
		t.Fatalf("seed mixed entry: %v", err)
	}

	svc := newReadStack(pool)
	opts, err := svc.Filters(context.Background(), owner, nil)
	if err != nil {
		t.Fatalf("filters: %v", err)
	}
	byID := map[uuid.UUID]domain.FilterParticipant{}
	for _, p := range opts.Participants {
		byID[p.ID] = p
	}
	wantRoles := map[uuid.UUID]string{
		owner:      string(domain.ActorRoleOwner),
		fullAccess: string(domain.ActorRoleFullAccess),
		viewer:     string(domain.ActorRoleViewer),
		exited:     string(domain.ActorRoleFullAccess),
		mixed:      string(domain.ActorRoleFullAccess),
	}
	tests := []struct {
		name      string
		id        uuid.UUID
		wantOwner bool
	}{
		{"владелец области", owner, true},
		{"full access member", fullAccess, false},
		{"viewer member", viewer, false},
		{"exited actor by journal snapshot", exited, false},
		{"journal snapshot vs membership — wider wins", mixed, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := byID[tt.id]
			if p.Role != wantRoles[tt.id] {
				t.Errorf("role: want %q, got %+v", wantRoles[tt.id], p)
			}
			if p.IsOwner != tt.wantOwner {
				t.Errorf("is_owner: want %v, got %+v", tt.wantOwner, p)
			}
		})
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
