import type { Metadata, Viewport } from 'next';
import type { JSX, ReactNode } from 'react';
import { Inter, Manrope } from 'next/font/google';
import { ErrorReporter } from '@/shared/lib/error-reporting/ErrorReporter';
import { ScrollToTop } from '@/shared/lib/scroll/ScrollToTop';
import { I18nProvider } from '@/shared/providers/i18n-provider';
import { QueryProvider } from '@/shared/providers/query-provider';
import { ToastProvider } from '@/shared/ui/toast';
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
    },
};

export const viewport: Viewport = {
    width: 'device-width',
    initialScale: 1,
    // maximumScale is intentionally omitted: capping zoom breaks accessibility
    // (WCAG 1.4.4 Resize text). PWA standalone mode does not require it.
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
