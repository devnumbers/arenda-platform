import type { Metadata, Viewport } from 'next';
import type { JSX, ReactNode } from 'react';
import { BfcacheGuard } from '@/shared/lib/bfcache/BfcacheGuard';
import { ErrorReporter } from '@/shared/lib/error-reporting/ErrorReporter';
import { ScrollToTop } from '@/shared/lib/scroll/ScrollToTop';
import { I18nProvider } from '@/shared/providers/i18n-provider';
import { QueryProvider } from '@/shared/providers/query-provider';
import { ToastProvider } from '@/shared/ui/toast';
import splashManifest from '@/shared/lib/pwa/splash-manifest.json';
import '../shared/styles/tokens.css';
import './globals.css';

// Шрифты самохостятся: @font-face и переменные --font-inter/--font-manrope/
// --font-onest живут в globals.css, файлы — в public/fonts. Прелоуд основных
// срезов (latin + cyrillic, то же, что preload'ил next/font с
// subsets: ['latin', 'cyrillic']); latin-ext/cyrillic-ext доедут по
// unicode-range по мере надобности.
const fontPreloads = [
    'inter-latin',
    'inter-cyrillic',
    'manrope-latin',
    'manrope-cyrillic',
    'onest-latin',
    'onest-cyrillic',
] as const;

export const metadata: Metadata = {
    title: 'Рентли',
    description: 'Управление арендной недвижимостью',
    appleWebApp: {
        capable: true,
        title: 'Рентли',
        statusBarStyle: 'default',
        startupImage: splashManifest.map(({ href, media }) => ({ url: href, media })),
    },
    // Next.js 15+ stopped emitting the deprecated `apple-mobile-web-app-capable`
    // meta tag (vercel/next.js#74524): `appleWebApp.capable: true` now produces
    // only `mobile-web-app-capable`, which is enough for Android but leaves iOS
    // showing a black startup screen instead of the branded splash image. The
    // legacy tag is still required by WebKit to render `apple-touch-startup-image`,
    // so it is re-added here via `metadata.other`.
    other: {
        'apple-mobile-web-app-capable': 'yes',
    },
};

export const viewport: Viewport = {
    width: 'device-width',
    initialScale: 1,
    // maximumScale is intentionally omitted: capping zoom breaks accessibility
    // (WCAG 1.4.4 Resize text). iOS auto-zoom-on-focus is prevented instead by
    // keeping native form controls at font-size ≥ 16px (see --font-size-input in
    // shared/styles/tokens.css). Do NOT re-add maximumScale — it is not needed.
    // White so the macOS title-bar strip behind the traffic-light buttons and
    // the Android Chrome address bar / iOS status bar render white instead of
    // the brand blue. Pairs with apple-mobile-web-app-status-bar-style 'default'
    // below, which makes iOS pick dark status-bar text on the light background.
    themeColor: '#ffffff',
    viewportFit: 'cover',
    // Клавиатура уменьшает и layout viewport (клавиатура — «resizes-content»):
    // fixed-панель StickyBottomBar поднимается над клавиатурой сама на
    // Android Chrome 108+ / Firefox Android 133+ (#1151, research #1148 §B).
    // iOS Safari мету игнорирует — там панель поднимает VisualViewport-хук
    // (useVisualKeyboardInset в sticky-bottom-bar.tsx). Десктопным браузерам
    // мета безразлична, vh/vw под клавиатурой в размерах панели не используются.
    interactiveWidget: 'resizes-content',
    // Светлая схема сайта: мета в SSR-разметке парсится до первого кадра и
    // действует, даже если CSS ещё не загрузился (color-scheme — свойство
    // документа, а не стилей); далее дублируется color-scheme: light в
    // globals.css (:root). Без неё на тёмных системах Chromium/WebKit
    // рисуют тёмные UA-скроллбар и form-контролы.
    colorScheme: 'light',
};

export default function RootLayout({
    children,
}: Readonly<{
    children: ReactNode;
}>): JSX.Element {
    return (
        <html lang="ru">
            <body>
                {fontPreloads.map((font) => (
                    <link
                        key={font}
                        rel="preload"
                        href={`/fonts/${font}.woff2`}
                        as="font"
                        type="font/woff2"
                        crossOrigin="anonymous"
                    />
                ))}
                <ErrorReporter />
                <ScrollToTop />
                <BfcacheGuard />
                <I18nProvider locale="ru-RU">
                    <QueryProvider>
                        {children}
                        <ToastProvider />
                    </QueryProvider>
                </I18nProvider>
            </body>
        </html>
    );
}
