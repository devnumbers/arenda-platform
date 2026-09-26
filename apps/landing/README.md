# Landing — публичный лендинг Рентли

Единственная индексируемая поисковиками страница платформы: Next.js
(App Router, SSR/SSG) — перестроен с нуля по макетам Figma 1:1
(карта #888, ADR 0063; до этого здесь был Figma Make-экспорт на Vite).

## Стек и устройство

- **Next.js** (версия синхронна кабинету `apps/frontend`), `output: standalone`,
  React 19, TypeScript strict, **Tailwind v4** (CSS-first, токены в `@theme`).
- **Дизайн-токены** — 1:1 из Figma: шрифт Onest (те же woff2-срезы, что у
  кабинета, `public/fonts/`), палитра (`--color-primary #2b7fff`, `--color-ink
  #171a1c`, …) и типографическая шкала `Landing/*` (`text-h1…text-xs`).
- **Маршруты**: `/` (лендинг), `/privacy`, `/terms` (юрстраницы, контент в
  `lib/legal-content.ts`), `/healthz` (контейнерный healthcheck).
- **Авторизация/тариф**: серверный компонент хедера читает сессионную cookie и
  ходит на `GET /me` через `BACKEND_URL` (compose-сеть; локально — dev-бэк),
  fail-open в гостевой хедер; см. `lib/auth.ts` (появляется тикетом T2 карты).
- **Контент секций** — константы `lib/content.ts` (тарифы/FAQ по макету),
  не API: цены и тексты лендинга меняются релизом лендинга.

## Команды

```bash
make landing-install     # npm install
make landing-dev         # next dev (порт 3000; для слотовой разработки см. ниже)
make landing-build       # next build
make landing-lint        # eslint
make landing-typecheck   # tsc --noEmit
```

Для разработки против слотового бэка (параллельные ворктри): создать
`apps/landing/.env.local` со слотовым `BACKEND_URL=http://localhost:809<N>`
(порт бэка слота) — Next подхватит его в dev-режиме.

## Деплой-контракт (не ломать молча)

- Образ `landing` собирается из корня репо (`apps/landing/Dockerfile`,
  node standalone, `PORT=8080`, healthcheck `/healthz`); версии Node штампует
  `make versions-sync` (`FROM node:24-alpine`).
- Caddy-матчер `@landing` (`deploy/caddy/rentlee.caddy`): `/`, `/privacy`,
  `/terms`, `/robots.txt`, `/sitemap.xml`, `/_next/*`, `/fonts/*`, `/icon.png`.
  Новый публичный путь = файл в `public/` (или app-маршрут) **+ строка в
  матчере**.
- Smoke `_deploy.yml` ждёт от `/` маркер `id="landing-root"`, работающий
  `/_next/static/*.js` (immutable), `/icon.png` и `robots.txt` (text/plain).
- `robots.txt` и `sitemap.xml` — статика `public/`; `Disallow` зеркалит
  кабинетные префиксы (`apps/frontend/shared/lib/pwa/app-routes.ts` минус
  `/login`), `Sitemap` — прод-канон `https://rentlee.ru`.
- Аналитика (Яндекс.Метрика/Google): подключает владелец после влития карты
  #888; скрипты метрик потребуют расширения security-заголовков в
  `next.config.ts` (сейчас контур nginx-набор без CSP).

## Figma

Файл «📌 Рентли. Сервис и лендинг»: десктоп-фулл `2814-728`, мобайл-фулл
`2859-3454`, доска секций `2826-151222`; компоненты `2865-5326` (ссылки),
`2864-5099` (кнопки), `2865-5537` (этапы), `2865-5440` (футер-ссылки).
Адаптивы: ≥1200 / 481–1199 / ≤480.
