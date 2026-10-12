'use client';

import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { IconButton } from '@/shared/ui/design';

/**
 * «Назад» хаба объекта — детерминированно на список объектов, не по
 * истории: на хаб вверх ведут push'и (стрелка «Операций» объекта #472,
 * строка «Объект» страницы операции), и history.back() со страницы
 * объекта разворачивал обратно в ветку операций/платежей — список
 * объектов оставался недостижим (исключение в CODING_STANDARDS
 * «Navigation and browser history»). Живой экран и route-loading ведут
 * этой кнопкой одинаково.
 */
export function PropertyDetailBackButton(): JSX.Element {
  const router = useRouter();

  return (
    <IconButton
      icon={<ArrowLeft />}
      label="Назад"
      onClick={() => router.push(ROUTES.properties)}
    />
  );
}
