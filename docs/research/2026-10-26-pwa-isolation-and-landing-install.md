# PWA: изоляция standalone без персистентного флага + установка с лендинга

Дата: 2026-10-26
Вход для: карта https://github.com/devnumbers/arenda-platform/issues/1610 (PWA «Рентли» с лендинга). Полные конспекты — в resolution-комментариях тикетов [#1611](https://github.com/devnumbers/arenda-platform/issues/1611) (изоляция) и [#1612](https://github.com/devnumbers/arenda-platform/issues/1612) (установка); здесь — только решения и фактура для навигации по коду.

## Баг «залипшего флага» (Android Chrome)

SW кабинета (`apps/frontend/public/sw.js`, scope `/`) перехватывал навигации на `/` и по IndexedDB-флагу `rentli-pwa/pwa/standalone` отдавал 302 → `/properties`. Флаг писала страница (`standalone-store.ts` — снесён) только в `true`, сброса не существовало. Android WebAPK делит хранилище профиля с обычными вкладками Chrome — один запуск установленного приложения навсегда закрывал лендинг в браузере (iOS изолирован хранилищем, не воспроизводил).

**Решение:** персистентный флаг снесён целиком. SW принципиально не может узнать display-mode навигационного запроса (CSSWG #14125 предлагает `application-context` media feature именно поэтому; `Sec-Fetch-*`, `clients`, `launchQueue` — не несут признака). Изоляция — синхронный инлайн-скрипт в `<head>` layout лендинга (`apps/landing/app/layout.tsx`): `location.replace('/properties')` до paint, без вспышки и без хвоста в history. Приоритет `navigator.standalone === true` на WebKit (ложные срабатывания matchMedia на iOS — w3c/manifest#1092), matchMedia доверяем только при `standalone === undefined`. `activate` SW сносит легаси-БД `deleteDatabase('rentli-pwa')` у уже пострадавших; доставка нового SW — тихое обновление ADR 0032, bump `CACHE_VERSION` не требовался.

## Установка PWA с лендинга

Весь объём — metadata в `apps/landing/app/layout.tsx`: `manifest: "/manifest.webmanifest"` (линк на манифест кабинета — тот же app id «/», одна инсталляция; свой manifest.ts лендингу не заводить: в проде путь обслуживает кабинет по catch-all, `@landing` не расширялся) и `appleWebApp: { capable, title: "Рентли", statusBarStyle: "default" }` (имя под иконкой на iOS; standalone даёт манифест). Chrome: линка достаточно (SW для installability не требуется с 2021; при уже установленном приложении промпт скрыт сам). Инструкции-модалки лендинга (`install-instructions.tsx`) после этого становятся рабочими; нативная beforeinstallprompt-кнопка отклонена владельцем (UX утверждён макетом).

## Иконка Android (safe zone)

Прежний `icon-maskable-512.png` (прогон RFG 21.09) выводил арт за safe zone (макс. радиус ~1.0 при лимите 0.8) — лончеры резали дорожку, иконка выглядела «крупной». Замещён композицией дизайнера `apps/frontend/assets/icons/source-maskable-android.png` (дом в круге 0.63, дорожка внутри холста); вид на устройстве одобрен владельцем 26.10. Процесс и маппинг — `apps/frontend/assets/icons/README.md`; канон поверхностей — `docs/research/pwa-icons-canon.md`.

## Открытые хвосты

- Расхождение research о старте A2HS с лендинга (URL-источник vs `start_url`) — на дизайн не влияет (гард ловит любой лендинг-путь), сверить на живом iOS.
- Затенение иконок в проде: `@landing` отдаёт `/icon.png`, `/apple-icon.png`, `/favicon.ico` лендинга — кабинетские file-convention иконки затенены (md5-подтверждено в #1612); кандидат в отдельный тикет.
- Глоу-кант и прозрачные углы Android-maskable — отступление от канона «непрозрачный полный квадрат», принятое владельцем по виду на устройстве; при жалобах на лончерах с маской меньше радиуса скругления — просить у дизайнера экспорт без глоу.
