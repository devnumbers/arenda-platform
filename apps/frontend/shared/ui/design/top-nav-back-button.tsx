'use client';

import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft } from '@/shared/assets/icons';
import { goBack } from '@/shared/lib/navigation';
import { IconButton } from './icon-button';

export type TopNavBackButtonProps = {
  /** Куда вести при отсутствии истории (прямой заход в свежей вкладке) —
   * та же семантика, что у легаси-BackButton: history-first, фолбэк —
   * router.replace(fallbackHref). */
  readonly fallbackHref: string;
  readonly label?: string;
};

/** Ведущая кнопка «Назад» подэкрана (Figma 529:3934, иконка 24 в зоне 44)
 * для слота leading у TopNav. Клиентский компонент с сериализуемыми
 * пропами: страница-серверный компонент собирает ею подэкранный хедер
 * без собственного 'use client' (goBack требует роутер). */
export function TopNavBackButton({
  fallbackHref,
  label = 'Назад',
}: TopNavBackButtonProps): JSX.Element {
  const router = useRouter();

  return (
    <IconButton icon={<ArrowLeft />} label={label} onClick={() => goBack(router, fallbackHref)} />
  );
}
