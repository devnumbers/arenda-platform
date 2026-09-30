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
  images: {
    // AVIF для картинок: меньше LCP-полезной нагрузки при том же качестве.
    formats: ["image/avif", "image/webp"],
    // Качество выдачи 90: дефолтные 75 дают видимые артефакты на фото;
    // все запросы (включая дефолтные 75) сходятся к ближайшему из списка.
    qualities: [90],
    // Лестница ширин без 3840: каждый вариант гуляет в srcset каждой
    // картинки (и в RSC-пейлоад), а после деплоя энкодится на лету —
    // 3840 из multi-мегапиксельных исходников (hero 3750px, стрип 4096px)
    // был самым тяжёлым холодным энкодом первой загрузки (карта #888).
    // Потолок 1920: retina-слоты 960–1024 CSS получают его вместо 2048/3840,
    // для маркетинговой страницы разница незаметна (решение владельца 30.09).
    deviceSizes: [640, 750, 828, 1080, 1200, 1920],
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
