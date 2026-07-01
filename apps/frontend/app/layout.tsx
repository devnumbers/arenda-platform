import type { Metadata } from 'next';
import type { JSX } from 'react';
import { Inter } from 'next/font/google';
import { ToastContainer } from 'react-toastify';
import { I18nProvider } from '@/shared/providers/i18n-provider';
import { QueryProvider } from '@/shared/providers/query-provider';
import '../shared/styles/tokens.css';
import 'react-toastify/dist/ReactToastify.css';
import './globals.css';

const inter = Inter({
  variable: '--font-inter',
  subsets: ['latin', 'cyrillic'],
  display: 'swap',
});

export const metadata: Metadata = {
  title: 'Рентли',
  description: 'Управление арендной недвижимостью',
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>): JSX.Element {
  return (
    <html lang="ru" className={inter.variable}>
      <body>
        <I18nProvider locale="ru-RU">
          <QueryProvider>
            {children}
            <ToastContainer position="bottom-right" />
          </QueryProvider>
        </I18nProvider>
      </body>
    </html>
  );
}
