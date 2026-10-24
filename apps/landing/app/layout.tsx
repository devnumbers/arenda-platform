import type { Metadata, Viewport } from "next";
import { SmoothScroll } from "@/components/smooth-scroll";
import "./globals.css";

export const metadata: Metadata = {
  metadataBase: new URL("https://rentlee.ru"),
  title: "Рентли — сервис учёта аренды недвижимости",
  description:
    "Рентли — учёт аренды для собственников и небольшого арендного бизнеса: объекты, договоры и платежи в одном месте, напоминания об оплатах, совместный доступ и отчёты по итогам аренды.",
  alternates: {
    canonical: "/",
  },
  openGraph: {
    type: "website",
    locale: "ru_RU",
    url: "https://rentlee.ru/",
    siteName: "Рентли",
  },
  twitter: {
    card: "summary_large_image",
  },
};

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  // viewport-fit=cover: без него env(safe-area-inset-*) всегда 0 — нижний
  // паддинг vaul-шита (modal.tsx) не отъезжает от home-indicator.
  viewportFit: "cover",
  // Клавиатура Android Chrome сжимает layout viewport: поля формы в нижнем
  // шите остаются над клавиатурой (дефолт resizes-visual накрывает их).
  interactiveWidget: "resizes-content",
  themeColor: "#ffffff",
};

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="ru">
      <head>
        {/* Предзагрузка двух основных срезов Onest; ext-срезы доезжают по
            unicode-range при первом использовании. */}
        <link
          rel="preload"
          href="/fonts/onest-cyrillic.woff2"
          as="font"
          type="font/woff2"
          crossOrigin="anonymous"
        />
        <link
          rel="preload"
          href="/fonts/onest-latin.woff2"
          as="font"
          type="font/woff2"
          crossOrigin="anonymous"
        />
      </head>
      <body>
        {/* Карусель showcase до гидратации visibility:hidden — без JS
            показываем сразу. */}
        <noscript>
          <style>{`[data-carousel-canvas]{visibility:visible!important}`}</style>
        </noscript>
        <SmoothScroll />
        {children}
      </body>
    </html>
  );
}
