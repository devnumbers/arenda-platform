import type { Metadata } from 'next';
import type { ReactNode } from 'react';

export const metadata: Metadata = {
  title: 'Вход — Arenda Platform',
  description: 'Войдите по номеру телефона',
};

export default function LoginLayout({ children }: { readonly children: ReactNode }) {
  return children;
}
