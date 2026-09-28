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
 * пока useMe в полёте — скелетон, при ошибке/без имени и вне провайдера —
 * текстовый плейсхолдер.
 *
 * Раскладка центра — заголовок центрируется относительно всей полосы,
 * слоты leading/trailing наложены абсолютно по краям (Figma 1425:55798):
 * центр никогда не смещается от наличия или числа кнопок. Длинные
 * заголовки сжимаются с min-w-0 (truncate у TopNavTitle), не распирая
 * страницу на узких экранах.
 *
 * Крылья и ведущая кнопка (решение владельца 2026-09-08, аудит #563):
 * на планшетном ярусе (561–1023) у подэкранов с leading-кнопкой крылья
 * не рисуются; их хедер — мобильная анатомия (leading, центр, trailing)
 * и стоит во всю ширину вьюпорта: слоты у краёв (←24/тайтл по центру/⋮24,
 * макет 1185-40814, решение 25.09 при доработке #865). На ПК ≥1024
 * крылья всегда, слоты подэкрана — в колонке 560 (макет 1185:40819).
 * Хаб-экраны (без leading) держат крылья на всём планшетно-ПК диапазоне
 * (Figma 1603:93153); их компакт-слоты остаются в колонке 560 — края
 * вьюпорта на планшете заняты крыльями (решение 25.09, гриллинг
 * доработки #865, Q5-А). mobileWings и leading вместе не сочетаются:
 * mobileWings — признак хаба, хаб без leading. */
export type TopNavProps = {
  readonly leading?: ReactNode;
  readonly trailing?: ReactNode;
  /** Постоянный правый слот бара хаба (аудит #876): действие, живущее в
   * баре всегда, а не по скроллу компакта — кебаб/шестерёнка «Уведомлений»
   * (канон §2 «кебаб в правом слоте бара»). На ПК — у правого края колонки
   * 560 (макет 2329-148700), крыло профиля остаётся у края вьюпорта; на
   * мобайле/планшете слот встаёт левее крыла-аватара (имя крыла ниже ПК
   * скрыто — ширина крыла стабильна, клиренс константный). Отличать от
   * trailing подэкрана: тот живёт в мобильной анатомии без крыльев. */
  readonly barTrailing?: ReactNode;
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
  /** TopNav внутри полноэкранной поверхности (пикер, визард-оверлей, шит
   * фильтров — класс .fullscreen-surface): на мобайле и планшете крыльев
   * нет (анатомия подэкрана). На ПК ≥1024 крылья рисует сама поверхность —
   * хром ПК не исчезает под открытой поверхностью (решение владельца
   * 25.09, доработка #865): лого и профиль в тех же слотах, что у страниц;
   * дублей нет — страница под поверхностью накрыта целиком. Колонка TopNav
   * легает на колонку страницы (бары поверхности растянуты на весь
   * вьюпорт — globals.css). */
  readonly overlay?: boolean;
};

export function TopNav({
  leading,
  trailing,
  barTrailing,
  children,
  className,
  variant = 'default',
  mobileWings = false,
  collapse,
  overlay = false,
}: TopNavProps): JSX.Element {
  const router = useRouter();
  const user = useTopNavUser();

  const firstName = user?.name;
  // pending — useMe в полёте (провайдер пометил сам): скелетон вместо
  // текста — текстовый плейсхолдер «Пользователь» раздувал крыло и
  // наезжал на заголовок хаба на мобайле (аудит #876), ширина кнопки
  // должна быть стабильной. Ошибка useMe и имя-null — терминальные
  // состояния, вне провайдера контекста нет — все три показывают
  // текстовый плейсхолдер, как раньше.
  const pending = user !== null && user.pending === true && firstName === undefined;
  const wingName = pending ? undefined : (firstName ?? 'Пользователь');
  // Крылья: мобайл — только с mobileWings; планшет (561–1023) — без
  // leading-кнопки (иначе наложение на колонку 560, см. JSDoc), ПК — всегда.
  // Boolean — чтобы условный leading={cond && <Button/>} при cond=false
  // считался «без leading». Внутри полноэкранной поверхности (overlay)
  // крылья на мобайле/планшете отсутствуют, на ПК их рисует сама
  // поверхность — хром ПК не исчезает (решение 25.09, доработка #865).
  const hasLeading = Boolean(leading);
  const wingsMobileClass = !overlay && mobileWings ? 'flex' : 'hidden';
  const wingsTierClass = hasLeading || overlay ? 'desktop:flex' : 'tablet:flex';

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
        {/* Имя крыла — только на ПК (аудит #876): на мобайле/планшете
         * аватар-only делает ширину крыла константной, и постоянные слоты
         * бара (barTrailing) получают стабильный клиренс; имя доступно в
         * профиле и на ПК. */}
        <UserButton
          name={wingName}
          pending={pending}
          hideNameBelowDesktop
          onClick={() => router.push(ROUTES.profile)}
        />
      </div>
      {variant === 'search' ? (
        /* Поисковая шапка: слоты и поле — во всю ширину вьюпорта на мобайле
         * и планшете (макет 1185-40814, решение 25.09 — как у LoginShell),
         * на ПК — в колонке 560 (макет 1185:40819). */
        <div className="relative mx-auto flex h-[72px] w-full desktop:max-w-column items-center pl-3.5 pr-3.5">
          {leading !== undefined && <div className="flex shrink-0 items-center">{leading}</div>}
          <div className="ml-1 flex h-full min-w-0 flex-1 items-center">{children}</div>
        </div>
      ) : (
        /* Слоты — абсолютные слои по краям (Figma 1425:55798): центр
         * центрируется относительно всей полосы и не смещается от
         * наличия/отсутствия кнопок (правый слот всегда «зарезервирован»).
         * Кап колонки — только на ПК: на планшете слоты подэкрана (без
         * крыльев) стоят у краёв вьюпорта, тайтл — по центру вьюпорта
         * (макет 1185-40814, решение 25.09). */
        <div className="relative mx-auto h-[72px] w-full desktop:max-w-column">
          {leading !== undefined && (
            <div className="absolute left-0 top-0 flex h-full items-center pl-3.5">{leading}</div>
          )}
          {collapse ? (
            /* Компакт-бар: слоты (лупа, «+») — в колонке 560 на планшете и
             * ПК, потому что края вьюпорта там заняты крыльями (лого и
             * кнопка профиля) — решение 25.09, гриллинг доработки #865
             * (Q5-А); лупа скрыта на 561–800px — место занято крылом-лого.
             * На мобайле слой во всю ширину (капа нет) — слоты у краёв. */
            <div className="absolute inset-0 mx-auto h-full w-full tablet:max-w-column">
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
              {collapse.trailing !== undefined && (
                <div className="hub-compact absolute right-0 top-0 flex h-full items-center pr-3.5">
                  {collapse.trailing}
                </div>
              )}
            </div>
          ) : (
            <div className="flex h-full w-full min-w-0 items-center justify-center gap-2 px-3">{children}</div>
          )}
          {!collapse && trailing !== undefined && (
            <div className="absolute right-0 top-0 flex h-full items-center pr-3.5">
              {trailing}
            </div>
          )}
          {/* Постоянный правый слот бара хаба: на ПК — правый край колонки
           * 560 (макет 2329-148700); на мобайле/планшете с крыльями — левее
           * аватар-крыла (его ширина 72 = px-[14px] + круг 44 + px-[14px], зазор
           * 14 до слота). Прецедент канона §2 — «Уведомления» #876. */}
          {!collapse && barTrailing !== undefined && (
            <div
              className={cn(
                'absolute top-0 flex h-full items-center',
                mobileWings ? 'right-[72px] desktop:right-0 desktop:pr-3.5' : 'right-0 pr-3.5',
              )}
            >
              {barTrailing}
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
