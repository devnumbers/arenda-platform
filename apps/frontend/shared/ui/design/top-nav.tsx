'use client';

import type { JSX, ReactNode } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { ROUTES } from '@/shared/config/routes';
import { cn } from '@/shared/lib/cn';
import { HeaderLogo } from './header-logo';
import { UserButton } from './user-button';
import { useTopNavUser } from './top-nav-user-context';

/** Единый хедер новых экранов (Figma 1185:40818–19): одна белая полоса
 * 72 — центральная часть (Figma 948:48573 «Top Navigation») с слотами —
 * leading слева (кнопка «назад»), trailing справа (кнопки действий), в
 * центре children (Title+Subtitle, StepsChip, поиск или лого — композиция
 * экрана). Мобайл (560 и уже): полоса во всю ширину вьюпорта, в потоке
 * страницы — не закрепляется, уходит вместе со скроллом (футер снизу
 * закрепляет TabBar). Планшет и ПК (от 561): полоса закреплена над
 * прокруткой, центральная часть — колонка max-560 по центру (как контент
 * страницы), по краям вьюпорта — крылья: лого «Рентли» (отступ 22) и
 * кнопка профиля (отступ даёт её собственный паддинг 14). Имя в кнопке
 * приносит TopNavUserContext (источник — useMe в widgets/screen-layout);
 * до загрузки и при ошибке — плейсхолдер.
 *
 * Раскладка центра — заголовок центрируется относительно всей полосы,
 * слоты leading/trailing наложены абсолютно по краям (Figma 1425:55798):
 * центр никогда не смещается от наличия или числа кнопок. Длинные
 * заголовки сжимаются с min-w-0 (truncate у TopNavTitle), не распирая
 * страницу на узких экранах. */
export type TopNavProps = {
  readonly leading?: ReactNode;
  readonly trailing?: ReactNode;
  readonly children?: ReactNode;
  readonly className?: string;
  /** `search` — вариант поиска (Figma 706:12598): children (SearchField)
   * растягивается от кнопки leading до правого паддинга бара, trailing не
   * рисуется — его роль играет глиф внутри поля, сходящийся с позицией
   * кнопки поиска (36px от края), поэтому переключение не дёргается. */
  readonly variant?: 'default' | 'search';
  /** «Крылья» (лого + кнопка профиля) и на мобайле, не только на десктопе
   * (Figma 1733-27411 — глобальная лента «Задачи» #523): хаб-экраны без
   * leading-кнопки открываются шапкой хаба. */
  readonly mobileWings?: boolean;
};

export function TopNav({
  leading,
  trailing,
  children,
  className,
  variant = 'default',
  mobileWings = false,
}: TopNavProps): JSX.Element {
  const router = useRouter();
  const user = useTopNavUser();

  const firstName = user?.name;
  const displayName = firstName?.length ? firstName : 'Пользователь';
  const wingsClass = mobileWings ? 'flex' : 'hidden tablet:flex';

  return (
    <header
      aria-label="Навигация экрана"
      className={cn(
        'relative z-40 w-full bg-white font-sans pt-[env(safe-area-inset-top)]',
        'tablet:fixed tablet:inset-x-0 tablet:top-0',
        className,
      )}
    >
      <Link
        href={ROUTES.properties}
        aria-label="Объекты"
        className={cn(
          'absolute left-0 top-0 h-[72px] items-center pl-[22px] outline-none focus-visible:rounded-pill focus-visible:ring-2 focus-visible:ring-primary tablet:flex',
          wingsClass,
        )}
      >
        <HeaderLogo />
      </Link>
      <div className={cn('absolute right-0 top-0 h-[72px] items-center tablet:flex', wingsClass)}>
        <UserButton name={displayName} onClick={() => router.push(ROUTES.profile)} />
      </div>
      {variant === 'search' ? (
        <div className="relative mx-auto flex h-[72px] w-full tablet:max-w-[560px] items-center pl-3.5 pr-3.5">
          {leading !== undefined && <div className="flex shrink-0 items-center">{leading}</div>}
          <div className="ml-1 flex h-full min-w-0 flex-1 items-center">{children}</div>
        </div>
      ) : (
        /* Слоты — абсолютные слои по краям (Figma 1425:55798): центр
         * центрируется относительно всей полосы и не смещается от
         * наличия/отсутствия кнопок (правый слот всегда «зарезервирован»). */
        <div className="relative mx-auto h-[72px] w-full tablet:max-w-[560px]">
          {leading !== undefined && (
            <div className="absolute left-0 top-0 flex h-full items-center pl-3.5">{leading}</div>
          )}
          <div className="flex h-full w-full min-w-0 items-center justify-center gap-2 px-3">{children}</div>
          {trailing !== undefined && (
            <div className="absolute right-0 top-0 flex h-full items-center pr-3.5">{trailing}</div>
          )}
        </div>
      )}
    </header>
  );
}

/** Центральный блок TopNav варианта Title (Figma 934:19658): заголовок
 * 16/18 + необязательный подзаголовок 14/16 #6F787C. */
export type TopNavTitleProps = {
  readonly title: ReactNode;
  readonly subtitle?: ReactNode;
  readonly className?: string;
};

export function TopNavTitle({ title, subtitle, className }: TopNavTitleProps): JSX.Element {
  return (
    <span className={cn('flex min-w-0 flex-col items-center gap-0.5 text-center', className)}>
      <span className="truncate text-base font-medium text-content">{title}</span>
      {subtitle !== undefined && <span className="text-sm text-content-secondary">{subtitle}</span>}
    </span>
  );
}
