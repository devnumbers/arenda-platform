-- name: InsertActionJournal :one
INSERT INTO action_journal (
    id, property_id, actor_id, actor_role, actor_name, actor_email,
    kind, action, base_action, segments, searchable, context, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING id;

-- Чтение журнала (карта #704, тикет #708, ADR 0061 §7): страница ленты
-- «История действий» по объектам read-скоупа читателя. Область видимости —
-- SQL-функция actor_can_read_property (000142, до #884 —
-- actor_can_read_history 000137): владелец (включая архивные
-- объекты) и активные участники неархивных объектов; suspended и чужие
-- объекты строк не отдают (privacy-404 — существование не раскрывается).
--
-- Курсор двусторонний по ключу (created_at, id): before_ts/before_id —
-- страница СТАРШЕ ключа (скролл вверх), after_ts/after_id — страница МОЛОЖЕ
-- (prepend новых); аргументы пары ходят вместе, NULL — нет курсора. Порядок
-- DESC-запроса — (created_at DESC, id DESC); after-нога читается отдельным
-- ASC-запросом (ListActionJournalAfter), адаптер разворачивает её страницу
-- обратно — на проводе порядок строк всегда (created_at DESC, id DESC), и
-- переименование объекта не может увести строку из окна (канон #597).
--
-- Фильтры: csv-списки ('' = фильтра нет), период по created_at (верхняя
-- граница исключающая — экран считает датой+24ч), поиск — всегда-OR
-- предикат «как в Telegram» (ресерч #839, тикет #842): prefix-FTS
-- (plainto_tsquery санитизирует произвольный ввод, ':*' на хвосте делает
-- последнюю лексему префиксной; CASE-гард обязателен — to_tsquery(':*') на
-- пустом plainto бросает syntax error) OR ILIKE-trgm (подстроки, почты,
-- фрагменты внутри слова; q_trgm — экранированный паттерн, ESCAPE '\').
-- q_raw='' — поиска нет. Маршрутизация trgm/fts/both (#705) снесена: любая
-- маршрутизация — предсказание интента, обе ноги через GIN-индексы 000140
-- всегда дешевле неверного угадывания. Записи с обезличенным актёром
-- (actor_id IS NULL) под фильтр actor_ids не попадают.
--
-- name: ListActionJournal :many
SELECT aj.id,
       aj.property_id,
       p.name AS property_name,
       aj.actor_id,
       aj.actor_name,
       aj.actor_email,
       aj.actor_role,
       COALESCE(CASE WHEN u.photo_key IS NOT NULL
                     THEN '/api/v1/users/' || aj.actor_id::text || '/photo'
                     END, '')::text AS actor_photo_url,
       aj.kind,
       aj.action,
       aj.base_action,
       aj.segments,
       aj.context,
       aj.created_at
FROM action_journal aj
JOIN properties p ON p.id = aj.property_id
LEFT JOIN users u ON u.id = aj.actor_id
WHERE actor_can_read_property(aj.property_id, sqlc.arg('actor')::uuid)
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
       sqlc.arg('q_raw')::text = ''
       OR (
           aj.search_tsv @@ CASE
               WHEN plainto_tsquery('russian', sqlc.arg('q_raw')::text)::text = ''
                   THEN ''::tsquery
               ELSE to_tsquery('russian',
                    plainto_tsquery('russian', sqlc.arg('q_raw')::text)::text || ':*')
           END
           OR aj.searchable ILIKE '%' || sqlc.arg('q_trgm')::text || '%' ESCAPE '\'
       )
      )
ORDER BY aj.created_at DESC, aj.id DESC
LIMIT sqlc.arg('page_limit')::int;

-- After-нога двустороннего курсора (prepend новых, «новые снизу» ленты):
-- тот же скоуп, фильтры и поиск, что у ListActionJournal, окно «строго
-- новее якоря» (after_ts/after_id) — но от якоря В СТОРОНУ НЕПРЕРЫВНОСТИ:
-- (created_at ASC, id ASC) забирает старейшие строки окна, поэтому
-- after-цепочка продолжает ленту от якоря вверх и всплеск новых записей
-- больше страницы доносится целиком (контракт openapi «появившиеся записи
-- не теряются») — DESC-порядок здесь взял бы из всплеска новейшие и отрезал
-- старейший хвост. Один sqlc-запрос двух порядков не несёт; контрактный
-- DESC страницы возвращает адаптер разворотом. Before-пару запрос не принимает:
-- ноги курсора взаимоисключающие на сервисе.
--
-- name: ListActionJournalAfter :many
SELECT aj.id,
       aj.property_id,
       p.name AS property_name,
       aj.actor_id,
       aj.actor_name,
       aj.actor_email,
       aj.actor_role,
       COALESCE(CASE WHEN u.photo_key IS NOT NULL
                     THEN '/api/v1/users/' || aj.actor_id::text || '/photo'
                     END, '')::text AS actor_photo_url,
       aj.kind,
       aj.action,
       aj.base_action,
       aj.segments,
       aj.context,
       aj.created_at
FROM action_journal aj
JOIN properties p ON p.id = aj.property_id
LEFT JOIN users u ON u.id = aj.actor_id
WHERE actor_can_read_property(aj.property_id, sqlc.arg('actor')::uuid)
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
  AND (aj.created_at > sqlc.narg('after_ts')::timestamptz
       OR (aj.created_at = sqlc.narg('after_ts')::timestamptz
           AND aj.id > sqlc.narg('after_id')::uuid))
  AND (
       sqlc.arg('q_raw')::text = ''
       OR (
           aj.search_tsv @@ CASE
               WHEN plainto_tsquery('russian', sqlc.arg('q_raw')::text)::text = ''
                   THEN ''::tsquery
               ELSE to_tsquery('russian',
                    plainto_tsquery('russian', sqlc.arg('q_raw')::text)::text || ':*')
           END
           OR aj.searchable ILIKE '%' || sqlc.arg('q_trgm')::text || '%' ESCAPE '\'
       )
      )
ORDER BY aj.created_at ASC, aj.id ASC
LIMIT sqlc.arg('page_limit')::int;

-- Опции фильтров ленты (ADR 0061 §7, тикет #708): участники области =
-- владелец ∪ текущие участники (включая приостановленных — suspend не
-- вытирает их записи из журнала) ∪ все актёры журнала области (отозванные
-- и вышедшие остаются фильтруемыми — записи переживают отзыв). Живые имена
-- читаются из users: чип фильтра — метаданные UI, не строка журнала; снимки
-- строк остаются в самом журнале. Обезличенные актёры (пользователь удалён)
-- не фильтруемы по определению. Отображаемое имя собирает адаптер по канону
-- access.DisplayNameOf («Пользователь» вместо имени — карта #1105, аменд #1123).
-- is_owner (#711, макет 2067-163528): замок владельца — пользователь —
-- владелец ХОТЯ БЫ ОДНОГО объекта области (bool_or по ноге properties
-- UNION'а); приглашённый без своих объектов флага не получает. first_name —
-- имя без фамилии (u.name) для строки «(Вы)»; '' у безымянных (тогда
-- имя в чипе — «Пользователь»).
-- role (#840, макет 2184-94261): иконка роли строки шита — максимальный
-- доступ в области (MIN ранга: 0 owner, 1 full_access, 2 viewer); живая
-- нога property_members сильнее снимка журнала (actor_role) — она и есть
-- ранг строки members. role='owner' жёстко совпадает с is_owner: снимок
-- 'owner' от бывшего владельца в журнале замка не даёт (фолбэк в
-- full_access).
--
-- name: ListHistoryFilterParticipants :many
WITH scope_participant(user_id, is_owner, role_rank) AS (
    SELECT p.owner_id, TRUE, 0
    FROM properties p
    WHERE actor_can_read_property(p.id, sqlc.arg('actor')::uuid)
      AND (sqlc.arg('property_ids')::text = ''
           OR p.id = ANY(string_to_array(sqlc.arg('property_ids')::text, ',')::uuid[]))
    UNION
    SELECT m.user_id, FALSE, CASE WHEN m.role = 'full_access' THEN 1 ELSE 2 END
    FROM property_members m
    JOIN properties p ON p.id = m.property_id
    WHERE actor_can_read_property(m.property_id, sqlc.arg('actor')::uuid)
      AND (sqlc.arg('property_ids')::text = ''
           OR m.property_id = ANY(string_to_array(sqlc.arg('property_ids')::text, ',')::uuid[]))
    UNION
    SELECT aj.actor_id, FALSE,
           CASE aj.actor_role WHEN 'owner' THEN 0 WHEN 'full_access' THEN 1 ELSE 2 END
    FROM action_journal aj
    JOIN properties p ON p.id = aj.property_id
    WHERE actor_can_read_property(aj.property_id, sqlc.arg('actor')::uuid)
      AND (sqlc.arg('property_ids')::text = ''
           OR aj.property_id = ANY(string_to_array(sqlc.arg('property_ids')::text, ',')::uuid[]))
      AND aj.actor_id IS NOT NULL
)
SELECT u.id, u.name, u.surname, u.phone, u.email,
       COALESCE(CASE WHEN u.photo_key IS NOT NULL
                     THEN '/api/v1/users/' || u.id::text || '/photo'
                     END, '')::text AS photo_path,
       COALESCE(u.name, '') AS first_name,
       bool_or(s.is_owner) AS is_owner,
       CASE WHEN bool_or(s.is_owner) THEN 'owner'
            WHEN MIN(s.role_rank) <= 1 THEN 'full_access'
            ELSE 'viewer' END AS role
FROM scope_participant s
JOIN users u ON u.id = s.user_id
GROUP BY u.id
ORDER BY u.name ASC NULLS LAST, u.surname ASC NULLS LAST, u.id ASC;

-- Объекты области для шита фильтров: те же свойства, что отдаёт лента,
-- плюс фото-аватар карточки (канон #582). С приватными фото (ADR 0065,
-- #1227) аватар один на объект и живёт в properties.photo_key; наружу —
-- относительный same-origin путь стриминга через бэкенд.
-- '' = фото нет (COALESCE: sqlc верит NOT NULL колонке, а CASE по NULL-ключу
-- промахивается в NULL). Сужение property_ids — тот же фильтр ленты.
--
-- name: ListHistoryFilterObjects :many
SELECT p.id,
       p.name,
       p.address,
       p.type,
       COALESCE(CASE WHEN p.photo_key IS NOT NULL
                     THEN '/api/v1/properties/' || p.id::text || '/photo'
                     END, '')::text AS photo_url
FROM properties p
WHERE actor_can_read_property(p.id, sqlc.arg('actor')::uuid)
  AND (sqlc.arg('property_ids')::text = ''
       OR p.id = ANY(string_to_array(sqlc.arg('property_ids')::text, ',')::uuid[]))
ORDER BY p.name ASC, p.id ASC;

