import type { Metadata, Viewport } from "next";
import { SmoothScroll } from "@/components/smooth-scroll";
import "./globals.css";

export const metadata: Metadata = {
  metadataBase: new URL("https://rentlee.ru"),
  title: "Рентли — сервис управления арендой недвижимости",
  description:
    "Управление арендой для собственников и небольшого арендного бизнеса: объекты, договоры и платежи в одном месте, напоминания об оплатах, совместный доступ и отчёты по итогам аренды.",
  alternates: {
    canonical: "/",
  },
  // Установка PWA с лендинга (карта #1610, research #1612): линк на манифест
  // кабинета — та же инсталляция (id «/», start_url /properties), что и из
  // кабинета; Chrome после этого показывает «Установить приложение» и делает
  // рабочими инструкции-модалки секции contact. Свой manifest.ts лендингу не
  // нужен — в проде путь обслуживает кабинет (deploy/caddy/rentlee.caddy).
  manifest: "/manifest.webmanifest",
  // iOS: имя «Рентли» под иконкой при «Добавить на экран Домой»; standalone
  // даёт сам манифест. Next эмитит стандартный mobile-web-app-capable.
  appleWebApp: {
    capable: true,
    title: "Рентли",
    statusBarStyle: "default",
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

// PWA-изоляция: в standalone-режиме весь лендинг недостижим — клиент уводится
// в кабинет до первой отрисовки (карта #1610, research #1611). Раньше это делал
// service worker по IndexedDB-флагу, но WebAPK на Android делит хранилище
// профиля с обычными вкладками — однажды записанный флаг навсегда закрывал
// лендинг в браузере; SW в принципе не знает display-mode навигации.
// Синхронный инлайн-скрипт первым в <head>: исполняется до paint, без вспышки
// лендинга; location.replace не оставляет лендинг в history. Приоритет —
// navigator.standalone на WebKit: у iOS известны ложные срабатывания
// matchMedia (w3c/manifest#1092), поэтому display-mode доверяем только вне
// WebKit, где standalone === undefined. В обычном браузере скрипт — no-op,
// try/catch — страховка экзотических сред: падение скрипта не должно мешать
// отрисовке лендинга.
const STANDALONE_GUARD_SCRIPT =
  "try{if(navigator.standalone===true||(navigator.standalone===undefined&&window.matchMedia('(display-mode: standalone)').matches))location.replace('/properties')}catch(e){}";

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
        {/* Изоляция standalone: до paint, до шрифтов, до чего-либо (см.
            комментарий у STANDALONE_GUARD_SCRIPT). Текстовым ребёнком, без
            dangerouslySetInnerHTML — запрещён контуром безопасности (канон
            json-ld.tsx): React рендерит детей script как сырой текст. */}
        <script>{STANDALONE_GUARD_SCRIPT}</script>
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
