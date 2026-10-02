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
  // Allowlist качеств оптимизатора (дефолт [75]): 90 — иллюстрации пустых
  // состояний (quality={90} в EmptyState/PropertySectionEmpty, #1002), чтобы
  // мягкие градиенты 3D-артов не покрывались артефактами q75; 100 — иконки
  // шита выбора типа платежа (quality={100}, #1070: «фото лучшего качества» —
  // перекодировка выдачи без потерь поверх исходников q100).
  images: { qualities: [75, 90, 100] },
  // Next 16 блокирует дев-ресурсы (/_next/hmr, чанки) с origin'ов вне списка:
  // стек ui-walkthrough живёт на http://127.0.0.1:3010 — пускаем этот хост
  // (localhost разрешён по умолчанию).
  allowedDevOrigins: ['127.0.0.1'],
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
