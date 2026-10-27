import type { NextConfig } from "next";
import { CSP_BASE_DIRECTIVES } from "./shared/lib/csp";

// CSP шаг 1 (решение #331, тикет #388): без nonce — 'unsafe-inline' только для
// script/style, остальные директивы строгие (общий хвост обеих полис —
// CSP_BASE_DIRECTIVES в shared/lib/csp.ts; шаг 2 — Report-Only в proxy.ts,
// тикет #406 — строится на той же базе). Источники: API ходит через
// same-origin route (app/api/[...path]), шрифты самохостятся из public/fonts
// (@font-face в app/globals.css, без next/font/google — см. комментарий там).
// blob: убран из img-src вместе с PhotoGrid (#399) — превью загружаемых фото
// были единственным его потребителем; возврат — осознанная правка здесь.
// 'unsafe-eval' в dev: React Refresh реконструирует стектрейсы через eval —
// https://nextjs.org/docs/app/guides/content-security-policy
const isDev = process.env.NODE_ENV === 'development';

const contentSecurityPolicy = [
  `script-src 'self' 'unsafe-inline'${isDev ? " 'unsafe-eval'" : ''}`,
  "style-src 'self' 'unsafe-inline'",
  ...CSP_BASE_DIRECTIVES,
].join('; ');

// Базовый security-контур браузера (решение #331): тот же смысловой набор несут
// nginx-конфиги admin и landing (без CSP — шаг 1 только на Next-приложениях).
const securityHeaders = [
  { key: 'Content-Security-Policy', value: contentSecurityPolicy },
  { key: 'X-Content-Type-Options', value: 'nosniff' },
  { key: 'X-Frame-Options', value: 'DENY' },
  { key: 'Referrer-Policy', value: 'no-referrer' },
  {
    key: 'Permissions-Policy',
    value:
      'accelerometer=(), camera=(), geolocation=(), gyroscope=(), magnetometer=(), microphone=(), payment=(), usb=()',
  },
];

const nextConfig: NextConfig = {
  output: 'standalone',
  reactCompiler: true,
  // Единственное качество оптимизатора — 90 (решение владельца 22.10.2026
  // «оставим 90»; прежний хвост 75/100 снесён). Важно: qualities — список-
  // разрешение, а НЕ дефолт; дефолт зашит в Next (get-img-props:
  // qualityInt || 75), и у <Image> без явного пропа запрос q=75 получает
  // 400. Поэтому канон — quality={90} на каждом next/image: пропуск ловится
  // в деве сразу битой картинкой, а не тихим q75-мылом на градиентах
  // (прецедент артефактов q75 — #1002).
  // minimumCacheTTL — нижняя граница жизни оптимизированных ответов
  // /_next/image в браузере (дефолт 60 секунд: каждая перезагрузка страницы
  // перевалидировала каждую иллюстрацию — «картинки моргают при
  // перезагрузке»). Источники public/images считаются неизменяемыми:
  // новая версия иллюстрации — новое имя файла (канон #865), смены
  // содержимого под тем же именем быть не должно.
  images: { qualities: [90], minimumCacheTTL: 60 * 60 * 24 * 365 },
  // Next 16 блокирует дев-ресурсы (/_next/hmr, чанки) с origin'ов вне списка:
  // стек ui-walkthrough живёт на http://127.0.0.1:3010 — пускаем этот хост
  // (localhost разрешён по умолчанию). Туннель приёмки на телефоне — tuna
  // (тикет #1151): резервед-домен аккаунта *.ru.tuna.am — '*' покрывает ровно
  // один лейбл (424334-… .ru.tuna.am); формат временных туннелей tuna сменился
  // на <случай>.a.2na.am — пускаем и его (субдомен один лейбл, приёмка
  // #1438); опция действует только в dev, production-стенд хосты не режет.
  allowedDevOrigins: ['127.0.0.1', '*.ru.tuna.am', '*.a.2na.am'],
  // Явно, а не по умолчанию Next (true с 13.5.1): двойной рендер эффектов в
  // dev — часть принятого бара качества (волна A, бар #330).
  reactStrictMode: true,
  poweredByHeader: false,
  headers: () => [
    {
      // The service worker script must always be fetched fresh so updates are
      // picked up promptly (byte-compare update check). `public/` is served
      // with max-age=0,must-revalidate by default; this reinforces no-store
      // specifically for /sw.js.
      source: '/sw.js',
      headers: [
        { key: 'Cache-Control', value: 'no-store' },
      ],
    },
    {
      source: '/:path*',
      headers: securityHeaders,
    },
    {
      // Бренд-ассеты считаются неизменяемыми (решение владельца 22.10.2026):
      // дефолт public/ (max-age=0, must-revalidate) заставлял браузер ходить
      // перевалидацией за каждым файлом при каждой перезагрузке. Год без
      // перевалидации; новая версия ассета — новое имя файла (канон #865).
      // Прямые потребители — CSS-фоны логина, unoptimized-иконки категорий
      // уведомлений и сплеши PWA; пути next/image (/_next/image) кешируются
      // через minimumCacheTTL выше — headers() на них не действует.
      source: '/images/:path*',
      headers: [{ key: 'Cache-Control', value: 'public, max-age=31536000, immutable' }],
    },
    {
      // Immutable год — смена арта под тем же URL до клиентов не доезжает
      // (кэш браузера и WebAPK; переустановка PWA HTTP-кэш не чистит):
      // новое содержимое = новый ?v= у потребителей URL (app/manifest.ts,
      // push-payload.ts, sw.js; guard icon-urls.test.ts, README иконок).
      source: '/icons/:path*',
      headers: [{ key: 'Cache-Control', value: 'public, max-age=31536000, immutable' }],
    },
  ],
  turbopack: {
    rules: {
      '*.svg': {
        loaders: [
          {
            loader: '@svgr/webpack',
            options: {
              svgoConfig: {
                plugins: [
                  {
                    name: 'preset-default',
                    params: {
                      overrides: {
                        removeViewBox: false,
                      },
                    },
                  },
                  'prefixIds',
                ],
              },
            },
          },
        ],
        as: '*.js',
      },
    },
  },
};

export default nextConfig;
