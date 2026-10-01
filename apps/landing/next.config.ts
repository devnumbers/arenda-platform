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
    // Качество выдачи 95 (решение владельца 01.10, карта #1010, отменяет
    // q90 от 30.09): максимум визуального качества с запасом по артефактам
    // на фото с мелким текстом; исходники — оригинальные байты заливок
    // Figma (webp q100), все значения пропов сходятся к 95.
    qualities: [95],
    // Лестница с retina-ступенями 2560/3072/4000 (решение владельца 01.10,
    // карта #1010, отменяет потолок 1920 от 30.09): retina-мониторы получают
    // нативное разрешение исходников (hero 4000px), 1x-экраны — свою ширину;
    // варианты выше собственной ширины картинки не эмитятся (статический
    // импорт знает intrinsic-размер).
    deviceSizes: [640, 750, 828, 1080, 1200, 1920, 2560, 3072, 4000],
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
