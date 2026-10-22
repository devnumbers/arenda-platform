import { type JSX, type ReactNode } from 'react';
import { BoldHome, SmallArrowDown, Star } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';
import {
  formatDayMonthWithYear,
  formatOverdueDays,
  PaymentRowButton,
} from '@/entities/payment';
import type { GlobalPayment, IsoDate, Payment, PaymentOperation } from '@/entities/payment';
import { propertyTypeIcons, type PropertyType } from '@/entities/property';
import { CircleIcon, CirclePhoto, circleIconRing, EmptyState, Skeleton, SkeletonListRow, skeletonRowWidths } from '@/shared/ui/design';
import { CategoryIcon, categoryStyle, type CategoryIconSurface } from '@/features/payment-categories';
import { daysOverdue } from '../lib/overdue-days';
import { paymentRowSubtitle, PAYMENT_COMPLETED_LABEL } from '../lib/payment-row-subtitle';

/**
 * Иконка категории строки правила глобальных платежей (#575): красная
 * точка при накопленной просрочке правила (overdueOperationCount фида),
 * галочка выделения для удаления — в правке избранного (#579 — пометка,
 * #814 — выделение зажатием/кликом). Поверхность канта: white —
 * строки на белом, muted — карточки на серых лентах главного экрана.
 */
export function GlobalPaymentRuleIcon({
  payment,
  check = false,
  surface = 'white',
}: {
  readonly payment: Pick<GlobalPayment, 'category' | 'overdueOperationCount'>;
  readonly check?: boolean;
  readonly surface?: CategoryIconSurface;
}): JSX.Element {
  const style = categoryStyle(payment.category.source, payment.category.slug);
  return (
    <CategoryIcon
      icon={style.icon}
      color={style.color}
      badge={payment.overdueOperationCount > 0 ? 'notification' : undefined}
      check={check}
      surface={surface}
    />
  );
}

/**
 * Круглый слот иконки карточки (44) для служебных глифов канона —
 * замыкающие «Все …»/«Показать все» (879:17555: синий круг, белая иконка),
 * карточки объектов (879:9712, аватар «Объектов» #582: белый круг, тёмная
 * иконка); красная точка просрочки — как у CategoryIcon. Иконки категорий
 * идут через CategoryIcon, это — только для не-категорийных глифов.
 */
export function GlobalCardIcon({
  children,
  variant = 'primary',
  hasNotification = false,
}: {
  readonly children: ReactNode;
  readonly variant?: 'primary' | 'white';
  readonly hasNotification?: boolean;
}): JSX.Element {
  return (
    <span
      className={cn(
        'relative flex h-11 w-11 shrink-0 items-center justify-center rounded-pill',
        circleIconRing.muted,
        variant === 'primary' ? 'bg-primary' : 'bg-surface',
      )}
    >
      <span
        className={cn(
          'flex h-6 w-6 items-center justify-center [&>svg]:h-6 [&>svg]:w-6',
          variant === 'primary' ? 'text-white' : 'text-[#D3D7D9]',
        )}
      >
        {children}
      </span>
      {hasNotification && (
        <span
          className={cn(
            'absolute left-0 top-0 h-2.5 w-2.5 rounded-full bg-danger',
            circleIconRing.muted,
          )}
          aria-hidden
        />
      )}
    </span>
  );
}

/**
 * Аватар объекта в глобальных платежах (#582): фото в круге 44 или глиф
 * типа объекта (Category Icon, карта #1217; без типа — Bold/Home).
 * Поверхность card — на серой карточке «Объектов» (белый круг, кант
 * серого — 890:29696); row — на белой странице поиска (серый круг #F3F4F6,
 * кант белого — 888:19370, паттерн SelectAvatar). hasNotification —
 * красная точка просрочки карточек объектов хаба «Платежей» (879:9712,
 * как у CategoryIcon): точка лежит вне клипающего фото круга —
 * overflow-hidden на CircleIcon срезал бы её кант.
 */
export function PaymentObjectAvatar({
  photoUrl,
  type,
  surface,
  hasNotification = false,
}: {
  readonly photoUrl: string | null;
  readonly type?: PropertyType;
  readonly surface: 'card' | 'row';
  /** Красная точка просрочки (карточки объектов хаба «Платежей», 879:9712). */
  readonly hasNotification?: boolean;
}): JSX.Element {
  // Выборка из статичного реестра, не вызов: react-hooks/static-components.
  const Glyph = type !== undefined ? propertyTypeIcons[type] : BoldHome;
  // Механика показа (битое фото — канон #1275/#1286, гард, ожидание) —
  // примитив CirclePhoto (#1423); хаб платежей — ленивая подача.
  return (
    <span className="relative flex h-11 w-11 shrink-0">
      <CircleIcon
        variant={surface === 'card' ? 'muted' : 'white'}
        aria-hidden
        className="overflow-hidden rounded-full"
      >
        <CirclePhoto
          photoUrl={photoUrl}
          loading="lazy"
          glyph={<Glyph className="h-6 w-6 text-[#D3D7D9]" />}
        />
      </CircleIcon>
      {hasNotification && (
        <span
          className={cn(
            'absolute left-0 top-0 h-2.5 w-2.5 rounded-full bg-danger',
            circleIconRing.muted,
          )}
          aria-hidden
        />
      )}
    </span>
  );
}

/**
 * Секции экрана «Платежи объекта» (Figma 1043:57610/1043:62920): серые
 * группы-карточки с заголовком-ссылкой и стрелкой, пустое состояние —
 * подзаголовок внутри карточки, строка платежа (подзаголовок — дата
 * следующего вхождения или «На паузе», звезда избранного). Секции страницы
 * платежа продолжают использовать PaymentsGroup; PaymentsSection —
 * новый вид секции главного экрана: клик по заголовку открывает страницу
 * секции, где виден весь список.
 */

// Заголовок секции — 20/24. text-left явный: заголовки-стрелки живут
// внутри <button>, а UA-стиль кнопки центрирует текст — на узком экране
// двухстрочный заголовок («Просроченные платежи») рендерился по центру
// карточки.
const headingClass = 'text-left text-xl font-semibold leading-6 text-content';
/** Пояснение секции (Mobile/Text/M/400, 1043:62920): 14/16, серый. */
const hintClass = 'text-sm leading-4 text-content-secondary';

/** Подзаголовок пустой истории платежа — один и тот же текст у блока
 * страницы платежа и у пустого подэкрана (решение владельца #1338;
 * payments/GLOSSARY.md, UI-copy exception). */
export const PAYMENT_HISTORY_EMPTY_HINT = 'Платежей еще не было';

/**
 * Секция-карточка «Платежей объекта» (1043:57610, 1043:62920): заголовок
 * слева, шеврон-стрелка — у правого края карточки (как в SectionHeader и
 * секциях страницы платежа), вся строка — кнопка на страницу секции; пока
 * строк нет — серый подзаголовок-пояснение под заголовком (14/16,
 * Mobile/Text/M/400).
 */
export function PaymentsSection({
  title,
  onOpen,
  openLabel,
  emptyHint,
  testId,
  children,
}: {
  readonly title: string;
  readonly onOpen: () => void;
  readonly openLabel: string;
  readonly emptyHint: string;
  readonly testId: string;
  readonly children: ReactNode;
}): JSX.Element {
  const isEmpty = children === null;
  return (
    <section data-testid={testId} className="mx-6 rounded-card bg-surface-muted pb-3">
      <div className="px-6 pt-6">
        <button
          type="button"
          onClick={onOpen}
          aria-label={openLabel}
          className="pressable flex w-full cursor-pointer items-center justify-between gap-3 rounded-pill outline-none hover:opacity-80 focus-visible:ring-4 focus-visible:ring-primary"
        >
          <h2 className={headingClass}>{title}</h2>
          <SmallArrowDown className="-rotate-90 shrink-0 text-content-tertiary" aria-hidden />
        </button>
        {isEmpty && <p className={`${hintClass} mt-2 max-w-[360px]`}>{emptyHint}</p>}
      </div>
      {!isEmpty && <div className="mt-2 flex flex-col">{children}</div>}
    </section>
  );
}

/** Серая группа секции со строками (заголовок 20/24 сверху, строки приносят
 * свои 24px горизонтали); боковые поля 24 — поля экрана из Figma. Со
 * стрелкой-навигацией в заголовке (стрелки секций страницы платежа —
 * резолюция #452, все три секции со стрелками по макету 3214:76417,
 * #1194); кликабельна вся линия заголовка, не только шеврон. Пустое
 * состояние — серый подзаголовок внутри карточки, 14/16 Medium
 * (1127:31705; вес — 3214:76912). */
export function PaymentsGroup({
  title,
  open,
  emptyHint,
  children,
}: {
  readonly title: string;
  readonly open?: { readonly label: string; readonly onOpen: () => void };
  readonly emptyHint?: string;
  readonly children: ReactNode;
}): JSX.Element {
  const isEmpty = children === null;
  return (
    <section className="mx-6 rounded-card bg-surface-muted pb-3">
      <div className="px-6 pb-2 pt-6">
        {open !== undefined ? (
          <button
            type="button"
            onClick={open.onOpen}
            aria-label={open.label}
            className="pressable flex w-full cursor-pointer items-center justify-between rounded-pill outline-none hover:opacity-80 focus-visible:ring-4 focus-visible:ring-primary"
          >
            <h2 className={headingClass}>{title}</h2>
            <SmallArrowDown className="-rotate-90 shrink-0 text-content-tertiary" aria-hidden />
          </button>
        ) : (
          <h2 className={headingClass}>{title}</h2>
        )}
      </div>
      {isEmpty ? (
        emptyHint !== undefined ? (
          <p className={`${hintClass} px-6 pb-4 font-medium`}>{emptyHint}</p>
        ) : null
      ) : (
        children
      )}
    </section>
  );
}

/** Пустое состояние секции: заголовок «Нет …» и подсказка (853:17208). */
export function PaymentsEmptyCard({
  title,
  hint,
}: {
  readonly title: string;
  readonly hint: string;
}): JSX.Element {
  return (
    <section className="mx-6 rounded-card bg-surface-muted px-6 py-6">
      <h2 className={headingClass}>{title}</h2>
      <p className={`${hintClass} mt-2 max-w-[360px]`}>{hint}</p>
    </section>
  );
}

/** Пустое состояние целой страницы с иллюстрацией (1043:60174/60502):
 * картинка 128, заголовок и пояснение по центру — «Нет платежей», «Нет\n * просроченных платежей» и т.п. — на каноне EmptyState дизайн-слоя. */
export function PaymentsEmptyState({
  image,
  title,
  hint,
}: {
  readonly image: string;
  readonly title: string;
  readonly hint: string;
}): JSX.Element {
  return (
    <EmptyState imageSrc={image} imageRounded title={title} description={hint} />
  );
}

/** Карточка состояния с действием — ошибка загрузки с кнопкой «Повторить». */
export function PaymentsStateCard({
  title,
  hint,
  action,
}: {
  readonly title: string;
  readonly hint: string;
  readonly action?: ReactNode;
}): JSX.Element {
  return (
    <section className="mx-6 rounded-card bg-surface-muted px-6 py-6">
      <h2 className={headingClass}>{title}</h2>
      <p className={`${hintClass} mt-2`}>{hint}</p>
      {action !== undefined && <div className="mt-4">{action}</div>}
    </section>
  );
}

/** Заголовок секции платежей/операций — 16/500 серым; бока 24 и верх 24 —
 * своя вставка (строки списков приносят паддинг сами), снизу 0 — до списка
 * гэп 8 контейнера (канон Heading, Figma 1096-37789/1036-40829/1858-102657).
 * inset=false — без своей вставки ПО БОКАМ: глобальная лента (#541) держит
 * ритм 24px на уровне всей страницы, заголовки дат прижаты к её краю
 * (решение владельца 2026-09-05); верхние 24 — канон макета, действуют в
 * обоих вариантах. */
export function PaymentsHeading({
  children,
  inset = true,
}: {
  readonly children: ReactNode;
  readonly inset?: boolean;
}): JSX.Element {
  return <h2 className={inset ? `${headingClass} px-6 pt-6` : `${headingClass} pt-6`}>{children}</h2>;
}

/** Скелетон секции на время загрузки — серая карточка с пульсирующими
 * строками; геометрия реальной секции (§7 паритет): шапка 24 сверху и 8 до
 * строк, ряд 68 (иконка 44 + вертикаль 12/12), низ карточки 12. */
export function PaymentsSkeleton({ withHeading }: { readonly withHeading?: boolean }): JSX.Element {
  return (
    <section className="mx-6 rounded-card bg-surface-muted px-6 pt-6 pb-3" aria-hidden>
      <Skeleton className={`${withHeading ? 'mb-2' : ''} h-6 w-40 bg-surface-muted-hover`} />
      <div className="flex flex-col">
        <Skeleton className="h-17 bg-surface-muted-hover" />
        <Skeleton className="h-17 w-4/5 bg-surface-muted-hover" />
      </div>
    </section>
  );
}

/** Скелетон плоского списка строк правил (#605): строки канона
 * PaymentRowButton 24/12 (px-6 плоско, py-3 — иконка 44, название + объект,
 * справа сумма и дата; списки избранного #579 и просроченных #580).
 * description=false — строки без правого нижнего поля (сумма одной
 * строкой). */
export function PaymentsRowsSkeleton({
  rows = 5,
  description = true,
}: {
  readonly rows?: number;
  readonly description?: boolean;
}): JSX.Element {
  const widths = skeletonRowWidths(rows);
  return (
    <div aria-hidden className="flex flex-col">
      {widths.map((rowWidths, index) => (
        <SkeletonListRow
          key={index}
          value
          description={description}
          widths={rowWidths}
          className="py-3"
        />
      ))}
    </div>
  );
}

/** Просроченная операция (страница платежа #465, полный список #466 и
 * каталог объекта, фреймы 693:5435/850:15412/1332:61665): срок «на N дней»
 * и сумма красным, бейдж danger на иконке категории. Порядок asc — старейшая
 * первой, долг разбирают по порядку накопления. Поверхность: gray — секция
 * на сером блоке (кант серый), white — строки внутри страницы (кант белый).
 * onSelect ведёт на страницу операции. */
export function OverdueOperationRow({
  operation,
  today,
  variant = 'gray',
  className,
  onSelect,
}: {
  readonly operation: PaymentOperation;
  readonly today: IsoDate;
  readonly variant?: 'white' | 'gray';
  readonly className?: string;
  readonly onSelect?: () => void;
}): JSX.Element {
  const style = categoryStyle('default', operation.categorySlug);

  return (
    <PaymentRowButton
      className={cn('px-3', className)}
      variant={variant}
      danger
      categoryIcon={
        <CategoryIcon
          icon={style.icon}
          color={style.color}
          badge="danger"
          surface={variant === 'white' ? 'white' : 'muted'}
        />
      }
      title={operation.title}
      description={formatOverdueDays(daysOverdue(operation.date, today))}
      amountKopecks={operation.amountKopecks}
      onSelect={onSelect}
    />
  );
}

/** Строка правила-платежа в секциях (1323:61133): подзаголовок — дата
 * следующего вхождения, «На паузе» (opacity по истории 22) или «Полностью
 * оплачен» при вычисляемой завершённости (#1341); звезда избранного —
 * ПЕРЕД подзаголовком и во всех состояниях; у платежа с накопленной
 * просрочкой — красная точка-уведомление на иконке (State=Expired, обе
 * поверхности). Поверхность gray — строки на серых блоках (кант серый),
 * white — внутри страницы, открытой кликом по серому блоку (кант белый).
 * Выбор строки открывает страницу платежа (#465). */
export function PaymentRow({
  payment,
  today,
  variant = 'gray',
  hasOverdue = false,
  onSelect,
}: {
  readonly payment: Payment;
  readonly today: IsoDate;
  readonly variant?: 'white' | 'gray';
  readonly hasOverdue?: boolean;
  readonly onSelect?: () => void;
}): JSX.Element {
  const subtitle = paymentRowSubtitle(payment, today);
  const style = categoryStyle(payment.category.source, payment.category.slug);
  const paused = subtitle.kind === 'paused';

  const dateText =
    subtitle.kind === 'paused'
      ? 'На паузе'
      : subtitle.kind === 'completed'
        ? PAYMENT_COMPLETED_LABEL
        : subtitle.kind === 'date'
          ? formatDayMonthWithYear(subtitle.iso, today)
          : undefined;

  return (
    <PaymentRowButton
      variant={variant}
      className={paused ? 'opacity-60' : ''}
      categoryIcon={
        <CategoryIcon
          icon={style.icon}
          color={style.color}
          badge={hasOverdue ? 'notification' : undefined}
          surface={variant === 'white' ? 'white' : 'muted'}
        />
      }
      title={payment.title}
      subtitle={
        payment.isFavorite ? (
          <span className="inline-flex items-center gap-1">
            <Star className="h-4 w-4 shrink-0" aria-hidden />
            {dateText}
          </span>
        ) : (
          dateText
        )
      }
      amountKopecks={payment.amountKopecks}
      onSelect={onSelect}
    />
  );
}
