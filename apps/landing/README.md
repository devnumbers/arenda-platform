# Рентли — лендинг

Публичный лендинг «Рентли». Standalone SPA на Vite + React, экспорт из Figma Make.

## Запуск

```bash
npm install
npm run dev
```

## Сборка

```bash
npm run build
```

## Инфраструктура

- Отдаётся nginx из контейнера (порт 8080).
- Caddy публикует его на `/` как публичный сайт; кабинет — `apps/frontend`.

## Примечание

Тексты и визуал правятся через ре-экспорт из Figma Make или точечно в `src/app/components/`.
