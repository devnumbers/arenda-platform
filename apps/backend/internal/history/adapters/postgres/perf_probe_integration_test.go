//go:build integration

package postgres_test

// Перф-проба чтения журнала (карта #704, тикеты #708/#842; требование
// тикета — «перф-проба на сиде 100k+ строк, замер p95 ленты/поиска»).
// Сеет 120 000 синтетических строк с человеческим словарём за 3 года, затем
// меряет p50/p95 страниц ленты (первая/глубокая/скоупы/фильтры) и поисковых
// запросов всегда-OR предиката — prefix-FTS OR ILIKE-trgm (ресерч #839).
//
// В CI не участвует: запуск вручную —
//	HISTORY_PERF_PROBE=1 go test -tags=integration -run TestHistoryPerfProbe \
//		./internal/history/adapters/postgres/ -v -timeout 30m

import (
	"context"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	accesspg "github.com/nambers/arenda-planform/apps/backend/internal/access/adapters/postgres"
	accessapp "github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	historypg "github.com/nambers/arenda-planform/apps/backend/internal/history/adapters/postgres"
	historyapp "github.com/nambers/arenda-planform/apps/backend/internal/history/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/testdb"
)

func TestHistoryPerfProbe(t *testing.T) {
	t.Parallel()
	if os.Getenv("HISTORY_PERF_PROBE") == "" {
		t.Skip("перф-проба: задаётся вручную через HISTORY_PERF_PROBE=1")
	}
	pool := testdb.Setup(t)
	ctx := context.Background()

	owner := seedPerfJournal(t, pool)
	// Тот же стек, что в композиционном корне: читает владелец сида.
	policy := accessapp.NewMembershipPolicy(accesspg.NewOwnerResolver(pool), accesspg.NewMembershipRepository(pool))
	svc := historyapp.NewHistoryReadService(historypg.NewReadStore(pool), policy)

	// Скоупы пробы: один объект и один актёр из сида.
	var propertyID, actorID uuid.UUID
	if err := pool.QueryRow(ctx,
		`SELECT property_id, actor_id FROM action_journal LIMIT 1`,
	).Scan(&propertyID, &actorID); err != nil {
		t.Fatalf("pick scope: %v", err)
	}

	// Участник одного объекта из сорока: цена предиката actor_can_read_property
	// для не-владельца — отсекает 39/40 объектов сида.
	member := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, phone, role, name, surname, email, timezone)
		 VALUES ($1, '+70000000001', 'owner', 'Пётр', 'Участников', 'member@example.com', 'UTC')`, member,
	); err != nil {
		t.Fatalf("seed member: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO property_members (id, property_id, user_id, role, granted_by)
		 VALUES ($1, $2, $3, 'viewer', $4)`,
		uuid.Must(uuid.NewV7()), propertyID, member, owner,
	); err != nil {
		t.Fatalf("seed membership: %v", err)
	}

	period := time.Now().Add(-30 * 24 * time.Hour)
	scenarios := []struct {
		name string
		run  func() error
	}{
		{"feed_first_page", func() error {
			_, err := svc.Feed(ctx, owner, historyapp.FeedQuery{Limit: 50})
			return err
		}},
		{"feed_deep_pages", func() error {
			// 50 последовательных страниц вниз по времени: p95 по всем —
			// проверка постоянства стоимости страницы по глубине (research §6).
			cursor := ""
			for range 50 {
				page, err := svc.Feed(ctx, owner, historyapp.FeedQuery{Limit: 50, BeforeCursor: cursor})
				if err != nil {
					return err
				}
				if page.NextCursor == "" {
					break
				}
				cursor = page.NextCursor
			}
			return nil
		}},
		{"feed_property_scope", func() error {
			_, err := svc.Feed(ctx, owner, historyapp.FeedQuery{Limit: 50, PropertyIDs: []uuid.UUID{propertyID}})
			return err
		}},
		{"feed_actor_scope", func() error {
			_, err := svc.Feed(ctx, owner, historyapp.FeedQuery{Limit: 50, ActorIDs: []uuid.UUID{actorID}})
			return err
		}},
		{"feed_kind_period", func() error {
			from := period
			_, err := svc.Feed(ctx, owner, historyapp.FeedQuery{Limit: 50, Kinds: []string{kindPayment}, DateFrom: &from})
			return err
		}},
		{"search_word", func() error {
			_, err := svc.Feed(ctx, owner, historyapp.FeedQuery{Limit: 50, Query: "оплачена"})
			return err
		}},
		{"search_wordform", func() error {
			_, err := svc.Feed(ctx, owner, historyapp.FeedQuery{Limit: 50, Query: "изменено"})
			return err
		}},
		{"search_multiword", func() error {
			_, err := svc.Feed(ctx, owner, historyapp.FeedQuery{Limit: 50, Query: "аренда январь"})
			return err
		}},
		{"search_email_fragment", func() error {
			_, err := svc.Feed(ctx, owner, historyapp.FeedQuery{Limit: 50, Query: "@example"})
			return err
		}},
		{"search_digits", func() error {
			_, err := svc.Feed(ctx, owner, historyapp.FeedQuery{Limit: 50, Query: "2007"})
			return err
		}},
		{"search_prefix_wide", func() error {
			// Префикс одной частой буквы — худший случай всегда-OR:
			// широкий bitmap и FTS-, и ILIKE-ноги (ресерч #839 — «%а%»,
			// приемлемо для v1, мониторить).
			_, err := svc.Feed(ctx, owner, historyapp.FeedQuery{Limit: 50, Query: "о"})
			return err
		}},
		{"feed_member_of_one", func() error {
			_, err := svc.Feed(ctx, member, historyapp.FeedQuery{Limit: 50})
			return err
		}},
		{"search_no_hits", func() error {
			_, err := svc.Feed(ctx, owner, historyapp.FeedQuery{Limit: 50, Query: "0000"})
			return err
		}},
	}

	const runs = 9
	for _, sc := range scenarios {
		// Прогрев: план и кэш должны быть тёплыми, как в проде под трафиком.
		if err := sc.run(); err != nil {
			t.Fatalf("%s warmup: %v", sc.name, err)
		}
		samples := make([]float64, 0, runs)
		for i := range runs {
			start := time.Now()
			if err := sc.run(); err != nil {
				t.Fatalf("%s run %d: %v", sc.name, i, err)
			}
			samples = append(samples, float64(time.Since(start).Microseconds())/1000.0)
		}
		sort.Float64s(samples)
		p50 := samples[len(samples)/2]
		p95 := samples[int(float64(len(samples))*0.95)-1]
		t.Logf("PERF %-22s p50=%8.2f ms  p95=%8.2f ms", sc.name, p50, p95)
	}
}

// seedPerfJournal сеет 40 объектов с владельцем и 25 актёрами, затем 120 000
// строк журнала одним INSERT..SELECT: словарь из восьми реальных шаблонов
// строк, даты — равномерно за 3 года, распределение по объектам/актёрам/
// шаблонам — детерминированными модулями. Имена актёров резолвятся в сид
// напрямую (кто их писал в журнал), searchable — та же конкатенация, что у
// рекордера.
func seedPerfJournal(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	start := time.Now()

	owner := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO users (id, phone, role, name, surname, email, timezone)
		 VALUES ($1, '+70000000000', 'owner', 'Иван', 'Иванов', 'ivan@example.com', 'UTC')`, owner,
	); err != nil {
		t.Fatalf("seed owner: %v", err)
	}
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO properties (id, owner_id, name, type, address, status)
		 SELECT uuidv7(), $1, 'Объект ' || g, 'apartment', 'Москва, Тверская ' || g, 'active'
		 FROM generate_series(1, 40) g`, owner,
	); err != nil {
		t.Fatalf("seed properties: %v", err)
	}
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO users (id, phone, role, name, surname, email, timezone)
		 SELECT uuidv7(), '+7999' || lpad(g::text, 7, '0'), 'owner',
		        (ARRAY['Пётр','Анна','Олег','Мария','Борис'])[1 + g % 5],
		        (ARRAY['Петров','Сидорова','Кузнецов','Ленина','Смирнов'])[1 + g % 5],
		        'user' || g || '@example.com', 'UTC'
		 FROM generate_series(1, 25) g`,
	); err != nil {
		t.Fatalf("seed actors: %v", err)
	}

	// Восьмишаблонный словарь: словоформы («изменено»), многословные
	// названия («Аренда за январь»), имена — покрытие всех ног поиска.
	ins := `
WITH props AS (
    SELECT id, row_number() OVER (ORDER BY name) - 1 AS rn FROM properties
), actors AS (
    SELECT id, name, surname, email, row_number() OVER (ORDER BY email) - 1 AS rn
    FROM users WHERE email LIKE 'user%@example.com'
), vocab(kind, action, base, text) AS (
    VALUES
        ('operation', 'operation.paid',      'completed', 'Операция оплачена: Коммунальные платежи'),
        ('operation', 'operation.created',   'added',     'Операция создана: Электричество'),
        ('property',  'property.renamed',    'changed',   'Название объекта изменено'),
        ('payment',   'payment.created',     'added',     'Платёж создан: Аренда за январь'),
        ('payment',   'payment.updated',     'changed',   'Платёж изменён: Аренда за февраль'),
        ('task',      'task.completed',      'completed', 'Задача выполнена: Заменить замки'),
        ('rental',    'rental.created',      'added',     'Аренда создана: Анна Сидорова'),
        ('contact',   'contact.created',     'added',     'Контакт создан: Анна Ленина')
), v AS (
    SELECT *, row_number() OVER () - 1 AS rn FROM vocab
)
INSERT INTO action_journal (
    id, property_id, actor_id, actor_role, actor_name, actor_email,
    kind, action, base_action, segments, searchable, context, created_at
)
SELECT
    uuidv7(),
    pr.id,
    ac.id,
    'owner',
    ac.name || ' ' || ac.surname,
    ac.email,
    vv.kind, vv.action, vv.base,
    jsonb_build_array(jsonb_build_object('text', vv.text)),
    vv.text || ' ' || ac.name || ' ' || ac.surname || ' ' || ac.email,
    '{}'::jsonb,
    now() - ((g % 1576800) || ' minutes')::interval
FROM generate_series(0, 119999) g
JOIN props pr ON pr.rn = g % (SELECT count(*) FROM props)
JOIN actors ac ON ac.rn = (g / 7) % (SELECT count(*) FROM actors)
JOIN v vv ON vv.rn = (g / 13) % (SELECT count(*) FROM v)`
	if _, err := pool.Exec(t.Context(), ins); err != nil {
		t.Fatalf("seed journal: %v", err)
	}
	var n int
	if err := pool.QueryRow(t.Context(), `SELECT count(*) FROM action_journal`).Scan(&n); err != nil {
		t.Fatalf("count journal: %v", err)
	}
	if n < 100000 {
		t.Fatalf("seed too small: %d rows", n)
	}
	t.Logf("PERF seed: %d rows in %s", n, time.Since(start).Round(time.Millisecond))
	return owner
}
