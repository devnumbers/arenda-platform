import type { Metadata } from 'next';
import type { JSX } from 'react';
import { Inter } from 'next/font/google';
import { ToastContainer } from 'react-toastify';
import { QueryProvider } from '@/shared/providers/query-provider';
import './globals.css';

const inter = Inter({
  variable: '--font-inter',
  subsets: ['latin', 'cyrillic'],
  display: 'swap',
});

export const metadata: Metadata = {
  title: 'Arenda Platform',
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
        <QueryProvider>
          {children}
          <ToastContainer position="bottom-right" />
        </QueryProvider>
      </body>
    </html>
  );
}
