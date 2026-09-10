'use client';

import type { JSX, ReactNode } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { ROUTES } from '@/shared/config/routes';
import { Search } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';
import { HeaderLogo } from './header-logo';
import { UserButton } from './user-button';
import { useTopNavUser } from './top-nav-user-context';

/** Компакт-бар хаба (решение владельца 2026-09-09, Figma 1603-93157 →
 * 1733-93740): при прокрутке хаб-шапки (заголовок + пилюля) бар показывает
 * свёрнутый состав — заголовок 16/18 по центру, лупа поиска в левом слоте
 * (там, где у хаба есть пилюля), «+» в правом слоте (где он есть в
 * хаб-шапке). Видимость управляется прогрессом `--hub-collapse` (пишет
 * HubCollapseAnchor), только opacity/transform. На планшете и ПК компакт
 * живёт в самом закреплённом баре, слоты — края центральной колонки 560,
 * «крылья» остаются по краям бара; на 561–800px лупа скрыта — её место
 * занято крылом-лого (решение владельца 2026-09-09). На мобайле, где бар
 * в потоке и уезжает, поверх проявляется отдельный закреплённый клон
 * (лупа у края — как в Figma). */
export type TopNavCollapse = {
  readonly title: ReactNode;
  /** Поисковая пилюля хаба — в компакт-баре её роль играет лупа. */
  readonly search?: { readonly href: string; readonly label: string };
  /** «+» из хаб-шапки (Объекты, Задачи) — докится в правый слот. */
  readonly trailing?: ReactNode;
};

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
 * страницу на узких экранах.
 *
 * Крылья и ведущая кнопка (решение владельца 2026-09-08, аудит #563):
 * на планшетном ярусе (561–1023) у подэкранов с leading-кнопкой крылья
 * не рисуются — колонка 560 начинается там же, где кончается лого, и
 * «крыло» наложилось бы на неё; их хедер — мобильная анатомия (leading,
 * центр, trailing) в закреплённой колонке. На ПК ≥1024 крылья всегда —
 * до колонки >=232px, наложения нет. Хаб-экраны (без leading) держат
 * крылья на всём планшетно-ПК диапазоне (Figma 1603:93153). mobileWings
 * и leading вместе не сочетаются: mobileWings — признак хаба, хаб без
 * leading. */
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
  /** Компакт-бар хаба: проявляется по прогрессу прокрутки хаб-шапки
   * (HubCollapseAnchor пишет `--hub-collapse`). */
  readonly collapse?: TopNavCollapse;
};

export function TopNav({
  leading,
  trailing,
  children,
  className,
  variant = 'default',
  mobileWings = false,
  collapse,
}: TopNavProps): JSX.Element {
  const router = useRouter();
  const user = useTopNavUser();

  const firstName = user?.name;
  const displayName = firstName?.length ? firstName : 'Пользователь';
  // Крылья: мобайл — только с mobileWings; планшет (561–1023) — без
  // leading-кнопки (иначе наложение на колонку 560, см. JSDoc), ПК — всегда.
  // Boolean — чтобы условный leading={cond && <Button/>} при cond=false
  // считался «без leading».
  const hasLeading = Boolean(leading);
  const wingsMobileClass = mobileWings ? 'flex' : 'hidden';
  const wingsTierClass = hasLeading ? 'desktop:flex' : 'tablet:flex';

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
          'absolute left-0 top-0 h-[72px] items-center pl-[22px] outline-none focus-visible:rounded-pill focus-visible:ring-2 focus-visible:ring-primary',
          wingsMobileClass,
          wingsTierClass,
        )}
      >
        <HeaderLogo />
      </Link>
      <div
        className={cn(
          'absolute right-0 top-0 h-[72px] items-center',
          wingsMobileClass,
          wingsTierClass,
        )}
      >
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
          {collapse ? (
            /* Компакт-бар на планшете и ПК — в самом закреплённом баре:
             * заголовок строго по центру колонки, лупа в левом слоте
             * (скрыта на 561–800px — место занято крылом-лого). */
            <>
              {collapse.search && (
                <div className="hub-compact absolute left-0 top-0 hidden h-full items-center pl-3.5 min-[800px]:flex">
                  <Link
                    href={collapse.search.href}
                    aria-label={collapse.search.label}
                    className="flex size-11 shrink-0 cursor-pointer items-center justify-center rounded-button outline-none focus-visible:ring-2 focus-visible:ring-primary"
                  >
                    <Search className="h-6 w-6 text-content" aria-hidden />
                  </Link>
                </div>
              )}
              <div className="hub-compact flex h-full w-full min-w-0 items-center justify-center px-3">
                <TopNavTitle title={collapse.title} />
              </div>
            </>
          ) : (
            <div className="flex h-full w-full min-w-0 items-center justify-center gap-2 px-3">{children}</div>
          )}
          {(collapse?.trailing ?? trailing) !== undefined && (
            <div
              className={cn(
                'absolute right-0 top-0 flex h-full items-center pr-3.5',
                collapse && 'hub-compact',
              )}
            >
              {collapse?.trailing ?? trailing}
            </div>
          )}
        </div>
      )}
      {collapse && (
        /* Мобайл: бар хаба в потоке и уезжает при прокрутке — компакт
         * проявляется отдельным закреплённым клоном (Figma 1733:93740). */
        <div className="hub-compact hub-compact-bar fixed inset-x-0 top-0 z-40 bg-white pt-[env(safe-area-inset-top)] font-sans tablet:hidden">
          <div className="relative mx-auto h-[72px] w-full">
            {collapse.search && (
              <Link
                href={collapse.search.href}
                aria-label={collapse.search.label}
                className="absolute left-0 top-0 flex h-full items-center pl-3.5 outline-none focus-visible:ring-2 focus-visible:ring-primary"
              >
                <Search className="h-6 w-6 text-content" aria-hidden />
              </Link>
            )}
            <div className="flex h-full items-center justify-center px-14">
              <TopNavTitle title={collapse.title} />
            </div>
            {collapse.trailing && (
              <div className="absolute right-0 top-0 flex h-full items-center pr-3.5">
                {collapse.trailing}
              </div>
            )}
          </div>
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
