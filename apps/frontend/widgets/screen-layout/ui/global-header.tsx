'use client';

import type { JSX } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useMe } from '@/features/auth';
import { ROUTES } from '@/shared/config/routes';
import { HeaderLogo, UserButton } from '@/shared/ui/design';

/** Глобальный top-header новых экранов (#460, Figma 1043:60173): белая
 * полоса во всю ширину вьюпорта — лого «Рентли» слева (отступ 22),
 * кнопка профиля справа (её собственный паддинг даёт отступ 14 от края);
 * по центру остаётся место колонке контента с её локальным TopNav.
 * Закреплён над прокруткой (Figma 1043:57612): контент-column получает
 * компенсацию высоты в ScreenLayout, липкий TopNav парковится под ним.
 * Виден только на десктопе (--breakpoint-desktop): на мобайле шапкой
 * экрана служит его TopNav. Имя в кнопке — имя собственника; до загрузки
 * и при ошибке — плейсхолдер. */
export function GlobalHeader(): JSX.Element {
  const router = useRouter();
  const { data: user } = useMe();

  const firstName = user?.name;
  const displayName = firstName?.length ? firstName : 'Пользователь';

  const openProfile = (): void => {
    router.push(ROUTES.profile);
  };

  return (
    <header
      aria-label="Глобальная навигация"
      className="fixed inset-x-0 top-0 z-40 hidden h-11 w-full items-center justify-between bg-white pl-[22px] desktop:flex"
    >
      <Link
        href={ROUTES.properties}
        aria-label="Объекты"
        className="rounded-pill outline-none focus-visible:ring-2 focus-visible:ring-primary"
      >
        <HeaderLogo />
      </Link>
      <UserButton name={displayName} onClick={openProfile} />
    </header>
  );
}
