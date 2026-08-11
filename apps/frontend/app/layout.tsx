import type { Metadata, Viewport } from 'next';
import type { JSX, ReactNode } from 'react';
import { Inter, Manrope } from 'next/font/google';
import { BootScreen } from '@/shared/lib/pwa/BootScreen';
import { ErrorReporter } from '@/shared/lib/error-reporting/ErrorReporter';
import { ScrollToTop } from '@/shared/lib/scroll/ScrollToTop';
import { I18nProvider } from '@/shared/providers/i18n-provider';
import { QueryProvider } from '@/shared/providers/query-provider';
import { ToastProvider } from '@/shared/ui/toast';
import splashManifest from '@/shared/lib/pwa/splash-manifest.json';
import '../shared/styles/tokens.css';
import './globals.css';

const inter = Inter({
    variable: '--font-inter',
    subsets: ['latin', 'cyrillic'],
    display: 'swap',
});

const manrope = Manrope({
    variable: '--font-manrope',
    subsets: ['latin', 'cyrillic'],
    weight: ['400', '500', '600', '700', '800'],
    display: 'swap',
});

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
    themeColor: '#2b7fff',
    viewportFit: 'cover',
};

export default function RootLayout({
    children,
}: Readonly<{
    children: ReactNode;
}>): JSX.Element {
    return (
        <html lang="ru" className={`${inter.variable} ${manrope.variable}`}>
            <body>
                {/*
                  Boot screen — branded splash present in the first server HTML
                  so the user never sees a white screen during the gap between
                  first paint and React hydration (especially visible on PWA
                  launch after the native splash is dismissed). Hidden via CSS
                  by BootScreen's useLayoutEffect adding `hydrated` to <body>;
                  suppressHydrationWarning tolerates the node being removed by
                  side effect. See shared/lib/pwa/BootScreen.tsx.
                */}
                <div id="boot-screen" suppressHydrationWarning>
                    <img
                        className="boot-icon"
                        src="/images/boot-icon.png"
                        alt=""
                        width={72}
                        height={72}
                    />
                    <img
                        className="boot-title"
                        src="/images/rentle-title.svg"
                        alt="Rentle"
                        width={130}
                    />
                    <div className="boot-spinner" aria-hidden="true" />
                </div>
                <BootScreen />
                <ErrorReporter />
                <ScrollToTop />
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
