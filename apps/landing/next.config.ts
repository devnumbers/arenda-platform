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
    // AVIF для картинок: меньше LCP-полезная нагрузка при том же качестве.
    formats: ["image/avif", "image/webp"],
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
