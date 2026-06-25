'use client';

import type { ReactNode } from 'react';
import { I18nProvider as AriaI18nProvider } from '@react-aria/i18n';

type I18nProviderProps = {
  readonly children: ReactNode;
  readonly locale?: string;
};

export function I18nProvider({ children, locale = 'ru-RU' }: I18nProviderProps) {
  return <AriaI18nProvider locale={locale}>{children}</AriaI18nProvider>;
}
