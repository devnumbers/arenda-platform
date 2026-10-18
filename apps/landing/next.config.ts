import type { NextConfig } from "next";

// Security-контур браузера — тот же смысловой набор, что нёс nginx.conf
// старого лендинга (решение #331, тикет #388); nginx-приложения идут без CSP,
// поэтому CSP здесь тоже нет. Кэширование HTML управляется рендерингом
// (динамическая дырка авторизованного хедера — см. ADR 0063), статика
// /_next/static иммутабельна силами Next.
const SECURITY_HEADERS = [
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "X-Frame-Options", value: "DENY" },
  { key: "Referrer-Policy", value: "no-referrer" },
  {
    key: "Permissions-Policy",
    value:
      "accelerometer=(), camera=(), geolocation=(), gyroscope=(), magnetometer=(), microphone=(), payment=(), usb=()",
  },
];

const nextConfig: NextConfig = {
  output: "standalone",
  reactStrictMode: true,
  poweredByHeader: false,
  // Ассеты публикуются под /landing/_next/*: кабинет — тоже Next.js и по
  // контракту прокси владеет общим /_next/* (карта #649; попытка отдать
  // лендингу весь /_next/* сломала кабинет на stage 30.09 — ADR 0063).
  // Снимает префикс Caddy (deploy/caddy/rentlee.caddy): контейнер по-прежнему
  // раздаёт честные /_next/*. Переменная задаётся только в Dockerfile —
  // в next dev и локальных сборках без Caddy пути остаются дефолтными.
  assetPrefix: process.env.LANDING_ASSET_PREFIX ?? undefined,
  images: {
    // Endpoint оптимизатора (/_next/image) НЕ следует за assetPrefix:
    // серверный (RSC) рендер всегда пишет дефолтный путь, поэтому лендинг
    // разводится с кабинетом по параметру url в Caddy
    // (@landing_optimizer в deploy/caddy/rentlee.caddy).
    // AVIF для картинок: меньше LCP-полезной нагрузки при том же качестве.
    formats: ["image/avif", "image/webp"],
    // Качество выдачи 90 (решение владельца 16.10, карта #1483, #1500):
    // вместо 95 — вес фото ~вдвое меньше при визуально неотличимой выдаче;
    // откат — при видимых артефактах на живой приёмке. Пропы держать
    // синхронно (hero: quality={90}) — большее значение молча прижимается
    // к ближайшему разрешённому с dev-предупреждением. Исходники —
    // оригинальные байты заливок Figma (webp q100).
    qualities: [90],
    // Лестница (решение владельца 16.10, карта #1483, #1499; прежде
    // «только 2к/4к» — карта #1010): браузер берёт минимальную ступень
    // ≥ ширины на экране × DPR — ступени 1280/1920 дают телефонам и
    // 1x-мониторам точные по ширине варианты вместо запаса ×2 (hero на
    // телефоне 474 → ~240 КБ, декод-площадь мобильных вариантов ×4);
    // 2560/4000 остаются потолком. Расширение — allowlist-надмножество:
    // старые URL валидны, инвалидация кеша не нужна; после вливания все
    // URL в разметке новые — ручной прогрев warm-images.mjs сразу после
    // деплоя (решение #1502).
    deviceSizes: [1280, 1920, 2560, 4000],
    // Нижний пол лестницы: иконки 24–52px и фото ≤384 CSS берут свои
    // компактные варианты и не тянут 2к.
    imageSizes: [256, 384],
  },
  // Dev-чек Origin (как во фронте): walkthrough-браузер ходит на 127.0.0.1.
  allowedDevOrigins: ["127.0.0.1"],
  async headers() {
    return [
      { source: "/:path*", headers: SECURITY_HEADERS },
      {
        source: "/robots.txt",
        headers: [{ key: "Cache-Control", value: "public, max-age=3600" }],
      },
      {
        source: "/sitemap.xml",
        headers: [{ key: "Cache-Control", value: "public, max-age=3600" }],
      },
    ];
  },
};

export default nextConfig;
