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
    // Качество выдачи 95 (решение владельца 01.10, карта #1010): максимум
    // визуального качества с запасом по артефактам на фото с мелким текстом;
    // исходники — оригинальные байты заливок Figma (webp q100), все значения
    // пропов сходятся к 95.
    qualities: [95],
    // Лестница «только 2к/4к» (решение владельца 05.10, карта #1010,
    // обсуждение #1013): браузер берёт минимальную ступень ≥ ширины на
    // экране × DPR, поэтому телефоны, планшеты и 1x-мониторы получают
    // вариант 2560 — запас в ~2 раза против точной ступени; цена по весу
    // (hero на телефоне 474 КБ против 237) принята владельцем осознанно.
    deviceSizes: [2560, 4000],
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
