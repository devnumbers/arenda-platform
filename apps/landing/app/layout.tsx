import type { Metadata, Viewport } from "next";
import "./globals.css";

export const metadata: Metadata = {
  metadataBase: new URL("https://rentlee.ru"),
  title: "Рентли — сервис учёта аренды недвижимости",
  description:
    "Рентли — учёт аренды для собственников и небольшого арендного бизнеса: объекты, договоры и платежи в одном месте, напоминания об оплатах, совместный доступ и отчёты по итогам аренды.",
};

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
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
        {/* Reveal-секции до гидратации прозрачны — без JS показываем сразу. */}
        <noscript>
          <style>{`[data-reveal]{opacity:1!important;transform:none!important}`}</style>
        </noscript>
        {children}
      </body>
    </html>
  );
}
