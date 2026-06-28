# Закрытие оставшихся MVP-гэпов

Дата: 2026-06-28

## Контекст

По итогам полного ревью сайта (`docs/reviews/2026-06-28-full-site-review.md`) остались
незакрытые функциональные и технические доработки. Этот документ фиксирует дизайн
решений перед реализацией.

## Принятые решения

- Страница списка аренд `/leases` не делается; переход на `/leases/[id]` — из карточки объекта.
- Редактирование аренды происходит на самой странице аренды (переключение режима «Просмотр / Редактирование»).
- Управление напоминаниями остаётся через операции.
- Редактирование регулярной операции — отдельная страница `/finance/operations/recurring/[id]/edit`.
- Телефоны шифруем детерминированным AEAD: один номер → один шифротекст, чтобы сохранить поиск по `phone`.
- Auth guard — Next.js middleware с серверной проверкой сессии через `/me`.
- `RealIP` заменяем на собственный middleware с env `TRUSTED_PROXIES`.
- Админка и корневая страница `/` не делаются.

## Архитектура

### 1. Страница аренды `/leases/[id]`

- Маршрут: `apps/frontend/app/(cabinet)/leases/[id]/page.tsx`.
- Виджет: `apps/frontend/widgets/leases/ui/LeaseDetailPage.tsx`.
- Режимы: просмотр и редактирование переключаются на странице.
- Данные: `GET /leases/{id}`, `GET /operations?lease_id={id}` (фильтр добавляем), `PATCH /leases/{id}`, действия `complete` / `deposit-return`.
- Навигация: карточка аренды на странице объекта становится ссылкой на `/leases/{id}`.

### 2. Редактирование серии регулярной операций

- Маршрут: `apps/frontend/app/(cabinet)/finance/operations/recurring/[id]/edit/page.tsx`.
- Форма: все поля регулярной операции + обязательное поле «Применить с даты» (`apply_from_date`).
- Бэкенд: при `apply_from_date` старая серия закрывается датой перед ней, создаётся новая серия с обновлёнными параметрами, неотредактированные операции >= даты удаляются, исключения не трогаются.

### 3. Клиентский auth guard

- `apps/frontend/middleware.ts` проверяет `session_id` и делает серверный `fetch` к `/me` через `BACKEND_INTERNAL_URL`.
- При отсутствии валидной сессии — редирект на `/login`.

### 4. Детерминированное шифрование телефонов

- Расширяем `Encryptor` методами `DeterministicEncrypt/Decrypt`.
- Реализация: HKDF-SHA256 → два ключа (`encKey`, `macKey`); nonce = HMAC-SHA256(macKey, plaintext + purpose)[:12]; AES-256-GCM.
- Миграция добавляет `phone_encrypted BOOLEAN` в `users`, `sms_codes`, `login_attempts`.
- Репозитории шифруют на запись и расшифровывают на чтение; поиск идёт по зашифрованному значению.
- Backfill при старте приложения итерирует незашифрованные строки.

### 5. Proxy-aware IP extraction

- Новый middleware `realIPMiddleware` в `apps/backend/internal/platform/httpapi/real_ip_middleware.go`.
- `TRUSTED_PROXIES` — список CIDR; для локальной разработки `127.0.0.1/32,::1/128`.
- Из `X-Forwarded-For` берётся самый правый недоверенный IP.
- Заменяет `chi/middleware.RealIP` в `server.go`.

## Вне скоупа

- `/leases` (список), `/` (корневая страница), админка.
- Шифрование телефонов арендаторов (`tenant_contacts.phone`).
