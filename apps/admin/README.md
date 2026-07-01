# Arenda Admin

Отдельное административное приложение на React Admin + Vite.

## Предварительные требования

- Node.js и npm.
- Бэкенд запущен на `http://localhost:8080`.

## Переменные окружения

Приложение читает переменные из `.env`:

- `VITE_API_PREFIX` — префикс API (по умолчанию `/api`).

В dev-режиме запросы к `/api` проксируются на `http://localhost:8080` через `vite.config.ts`.

## Установка

```bash
make admin-install
# или
npm install
```

## Запуск

```bash
make admin-dev
# или
npm run dev
```

Приложение будет доступно по адресу `http://localhost:5173`.

## Сборка

```bash
make admin-build
# или
npm run build
npm run preview
```

## Проверка типов

```bash
make admin-typecheck
# или
npm run typecheck
```
