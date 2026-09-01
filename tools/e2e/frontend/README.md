# Frontend Playwright e2e

Экранные e2e тесты фронта: `apps/frontend/e2e` (`@playwright/test`). Обязательны
для новых экранов со сверкой дизайна с Figma (решение 2026-08-25, спека
[#453](https://github.com/devnumbers/arenda-platform/issues/453); инфраструктура —
тикет [#456](https://github.com/devnumbers/arenda-platform/issues/456)). Контракт
описан в `docs/testing-strategy.md`, раздел «Экранные e2e (Playwright)».

## Запуск

```bash
make frontend-e2e          # из корня репо; нужен Docker + Node + Go
```

Оркестратор `run-frontend-e2e.sh`:

1. поднимает postgres 18 (compose-проект `arenda-e2e`, порт 5436 — слот 0;
   слоты ворктри подменяют проект и порты через `.env`, см.
   `docs/agents/parallel-dev.md`; перед стартом сносит остатки прошлого
   прогона вместе с вольюмами);
2. собирает и запускает бекенд (`127.0.0.1:8081`): миграции применяются на
   старте, `EMAIL_SENDER=fake` (код входа пишется в JSON-лог),
   `PAYMENT_PROVIDER=fake`;
3. сеет данные SQL-сидом `seed.sql` (владелец, пред-аутентифицированная
   сессия, два активных объекта); криптографию сид-строк считает
   `e2e-crypto.mjs` — HMAC токена сессии и детерминированный шифротекст
   телефона (зеркалит `encryption.hashToken`/`DeterministicEncrypt`
   бекенда: `users.phone` ищется по шифротексту, не по открытому тексту);
4. собирает прод-билд фронта и стартует next standalone (`127.0.0.1:3010`,
   тот же рантайм, что в `apps/frontend/Dockerfile`);
5. гоняет `npx playwright test` и сносит стек (контейнеры, процессы).

Отчёт — `apps/frontend/playwright-report`, скриншоты —
`apps/frontend/test-results`; в CI джоба `frontend-e2e` выгружает их
артефактами для сверки с Figma.

## Переменные

| Переменная | Значение по умолчанию | Смысл |
| --- | --- | --- |
| `E2E_PG_PORT` | `5436` | порт postgres e2e-среды |
| `E2E_BACKEND_PORT` | `8081` | порт бекенда |
| `E2E_FRONTEND_PORT` | `3010` | порт фронта (он же `E2E_BASE_URL`) |
| `E2E_COMPOSE_PROJECT` | `arenda-e2e` | имя compose-проекта (имя контейнера postgres — `<проект>-postgres-1`) |
| `E2E_ENCRYPTION_KEY` | тестовый 64-hex ключ | ключ HMAC сид-сессии |
| `E2E_KEEP_STACK` | `0` | `1` — не сносить стек после прогона (отладка) |

Таргеты `make frontend-e2e*` сорсят корневой `.env` чекаута: слоты ворктри
(`arenda-e2e-wt<N>`, порты BASE+5+N) приходят в раннер из process env —
`docs/agents/parallel-dev.md`.

## Контракт тестов

Тестам оркестратор отдаёт: `E2E_BASE_URL`, `E2E_SESSION_TOKEN` (сырой токен
сид-сессии → кука `session_id`), `E2E_USER_PHONE` (цифры без +7),
`E2E_USER_EMAIL`, `E2E_BACKEND_LOG` (путь к логу бекенда для извлечения кода
входа). Фикстуры и хелперы — `apps/frontend/e2e/fixtures.ts`.
