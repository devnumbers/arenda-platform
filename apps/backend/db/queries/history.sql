-- name: InsertActionJournal :one
INSERT INTO action_journal (
    id, property_id, actor_id, actor_role, actor_name, actor_email,
    kind, action, base_action, segments, searchable, context, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING id;

-- Чтение журнала (карта #704, тикет #708, ADR 0061 §7): страница ленты
-- «История действий» по объектам read-скоупа читателя. Область видимости —
-- SQL-функция actor_can_read_history (000137): владелец (включая архивные
-- объекты) и активные участники неархивных объектов; suspended и чужие
-- объекты строк не отдают (privacy-404 — существование не раскрывается).
--
-- Курсор двусторонний по ключу (created_at, id): before_ts/before_id —
-- страница СТАРШЕ ключа (скролл вверх), after_ts/after_id — страница МОЛОЖЕ
-- (prepend новых); аргументы пары ходят вместе, NULL — нет курсора. Порядок
-- строк всегда (created_at DESC, id DESC) — переименование объекта не может
-- увести строку из окна (канон #597).
--
-- Фильтры: csv-списки ('' = фильтра нет), период по created_at (верхняя
-- граница исключающая — экран считает датой+24ч), поиск — маршрутизация
-- trgm/fts/both приложением (решение #705): q_trgm — экранированный
-- ILIKE-паттерн, q_raw — сырой ввод для websearch_to_tsquery (безопасен для
-- пользовательского ввода); mode='' — поиска нет. Записи с обезличенным
-- актёром (actor_id IS NULL) под фильтр actor_ids не попадают.
--
-- name: ListActionJournal :many
SELECT aj.id,
       aj.property_id,
       p.name AS property_name,
       aj.actor_id,
       aj.actor_name,
       aj.actor_email,
       aj.actor_role,
       aj.kind,
       aj.action,
       aj.base_action,
       aj.segments,
       aj.context,
       aj.created_at
FROM action_journal aj
JOIN properties p ON p.id = aj.property_id
WHERE actor_can_read_history(aj.property_id, sqlc.arg('actor')::uuid)
  AND (sqlc.arg('property_ids')::text = ''
       OR aj.property_id = ANY(string_to_array(sqlc.arg('property_ids')::text, ',')::uuid[]))
  AND (sqlc.arg('actor_ids')::text = ''
       OR aj.actor_id = ANY(string_to_array(sqlc.arg('actor_ids')::text, ',')::uuid[]))
  AND (sqlc.arg('kinds')::text = ''
       OR aj.kind = ANY(string_to_array(sqlc.arg('kinds')::text, ',')))
  AND (sqlc.arg('base_actions')::text = ''
       OR aj.base_action = ANY(string_to_array(sqlc.arg('base_actions')::text, ',')))
  AND (sqlc.narg('date_from')::timestamptz IS NULL
       OR aj.created_at >= sqlc.narg('date_from')::timestamptz)
  AND (sqlc.narg('date_to')::timestamptz IS NULL
       OR aj.created_at < sqlc.narg('date_to')::timestamptz)
  AND (sqlc.narg('before_ts')::timestamptz IS NULL
       OR aj.created_at < sqlc.narg('before_ts')::timestamptz
       OR (aj.created_at = sqlc.narg('before_ts')::timestamptz
           AND aj.id < sqlc.narg('before_id')::uuid))
  AND (sqlc.narg('after_ts')::timestamptz IS NULL
       OR aj.created_at > sqlc.narg('after_ts')::timestamptz
       OR (aj.created_at = sqlc.narg('after_ts')::timestamptz
           AND aj.id > sqlc.narg('after_id')::uuid))
  AND (
       sqlc.arg('mode')::text = ''
       OR (sqlc.arg('mode')::text = 'trgm'
           AND aj.searchable ILIKE '%' || sqlc.arg('q_trgm')::text || '%' ESCAPE '\')
       OR (sqlc.arg('mode')::text = 'fts'
           AND aj.search_tsv @@ websearch_to_tsquery('russian', sqlc.arg('q_raw')::text))
       OR (sqlc.arg('mode')::text = 'both'
           AND (aj.search_tsv @@ websearch_to_tsquery('russian', sqlc.arg('q_raw')::text)
                OR aj.searchable ILIKE '%' || sqlc.arg('q_trgm')::text || '%' ESCAPE '\'))
      )
ORDER BY aj.created_at DESC, aj.id DESC
LIMIT sqlc.arg('page_limit')::int;

-- Опции фильтров ленты (ADR 0061 §7, тикет #708): участники области =
-- владелец ∪ текущие участники (включая приостановленных — suspend не
-- вытирает их записи из журнала) ∪ все актёры журнала области (отозванные
-- и вышедшие остаются фильтруемыми — записи переживают отзыв). Живые имена
-- читаются из users: чип фильтра — метаданные UI, не строка журнала; снимки
-- строк остаются в самом журнале. Обезличенные актёры (пользователь удалён)
-- не фильтруемы по определению. Отображаемое имя собирает адаптер по канону
-- access.DisplayNameOf (маскированный телефон вместо сырого — PII).
--
-- name: ListHistoryFilterParticipants :many
SELECT DISTINCT u.id, u.name, u.surname, u.phone, u.email
FROM users u
WHERE u.id IN (
    SELECT p.owner_id
    FROM properties p
    WHERE actor_can_read_history(p.id, sqlc.arg('actor')::uuid)
      AND (sqlc.arg('property_ids')::text = ''
           OR p.id = ANY(string_to_array(sqlc.arg('property_ids')::text, ',')::uuid[]))
    UNION
    SELECT m.user_id
    FROM property_members m
    JOIN properties p ON p.id = m.property_id
    WHERE actor_can_read_history(m.property_id, sqlc.arg('actor')::uuid)
      AND (sqlc.arg('property_ids')::text = ''
           OR m.property_id = ANY(string_to_array(sqlc.arg('property_ids')::text, ',')::uuid[]))
    UNION
    SELECT aj.actor_id
    FROM action_journal aj
    JOIN properties p ON p.id = aj.property_id
    WHERE actor_can_read_history(aj.property_id, sqlc.arg('actor')::uuid)
      AND (sqlc.arg('property_ids')::text = ''
           OR aj.property_id = ANY(string_to_array(sqlc.arg('property_ids')::text, ',')::uuid[]))
      AND aj.actor_id IS NOT NULL
)
ORDER BY u.name ASC NULLS LAST, u.surname ASC NULLS LAST, u.id ASC;

-- Объекты области для шита фильтров: те же свойства, что отдаёт лента,
-- плюс фото-аватар карточки — первое по времени фото (канон #582);
-- '' = фото нет (COALESCE: sqlc верит NOT NULL колонке, а lateral LEFT
-- JOIN промахивается в NULL). Сужение property_ids — тот же фильтр ленты.
--
-- name: ListHistoryFilterObjects :many
SELECT p.id,
       p.name,
       p.address,
       COALESCE(ph.url, '') AS photo_url
FROM properties p
LEFT JOIN LATERAL (
    SELECT pp.url
    FROM property_photos pp
    WHERE pp.property_id = p.id
    ORDER BY pp.created_at, pp.id
    LIMIT 1
) ph ON true
WHERE actor_can_read_history(p.id, sqlc.arg('actor')::uuid)
  AND (sqlc.arg('property_ids')::text = ''
       OR p.id = ANY(string_to_array(sqlc.arg('property_ids')::text, ',')::uuid[]))
ORDER BY p.name ASC, p.id ASC;

