import type { JSX, ReactNode } from 'react';
import { ChevronDown, Star } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';
import {
  formatDayMonth,
  formatOverdueDays,
  PaymentRowButton,
} from '@/entities/payment';
import { EmptyState, Skeleton } from '@/shared/ui/design';
import type { IsoDate, Payment, PaymentOperation } from '@/entities/payment';
import { CategoryIcon, categoryStyle } from '@/features/payment-categories';
import { daysOverdue } from '../lib/overdue-days';
import { paymentRowSubtitle } from '../lib/payment-row-subtitle';

/**
 * Секции экрана «Платежи объекта» (Figma 1043:57610/1043:62920): серые
 * группы-карточки с заголовком-ссылкой и стрелкой, пустое состояние —
 * подзаголовок внутри карточки, строка платежа (подзаголовок — дата
 * следующего вхождения или «На паузе», звезда избранного). Секции страницы
 * платежа продолжают использовать PaymentsGroup; PaymentsSection —
 * новый вид секции главного экрана: клик по заголовку открывает страницу
 * секции, где виден весь список.
 */

const headingClass = 'text-xl font-semibold leading-6 text-content';
/** Пояснение секции (Mobile/Text/M/400, 1043:62920): 14/16, серый. */
const hintClass = 'text-sm leading-4 text-content-secondary';

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
    <section data-testid={testId} className="mx-6 rounded-card bg-surface-muted pb-6">
      <div className="px-6 pt-6">
        <button
          type="button"
          onClick={onOpen}
          aria-label={openLabel}
          className="flex w-full cursor-pointer items-center justify-between gap-3 rounded-pill outline-none transition-opacity hover:opacity-80 active:opacity-80 focus-visible:ring-4 focus-visible:ring-primary"
        >
          <h2 className={headingClass}>{title}</h2>
          <ChevronDown className="-rotate-90 shrink-0 text-content-tertiary" aria-hidden />
        </button>
        {isEmpty && <p className={`${hintClass} mt-2 max-w-[360px]`}>{emptyHint}</p>}
      </div>
      {!isEmpty && <div className="mt-2 flex flex-col">{children}</div>}
    </section>
  );
}

/** Серая группа секции со строками (заголовок 24/24 сверху, строки приносят
 * свои 24px горизонтали); боковые поля 24 — поля экрана из Figma. Со
 * стрелкой-навигацией в заголовке (стрелки секций страницы платежа —
 * резолюция #452); кликабельна вся линия заголовка, не только шеврон.
 * Пустое состояние — серый подзаголовок внутри карточки (1127:31705). */
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
    <section className="mx-6 rounded-card bg-surface-muted pb-6">
      <div className="px-6 pb-3 pt-6">
        {open !== undefined ? (
          <button
            type="button"
            onClick={open.onOpen}
            aria-label={open.label}
            className="flex w-full cursor-pointer items-center justify-between rounded-pill outline-none transition-opacity hover:opacity-80 active:opacity-80 focus-visible:ring-4 focus-visible:ring-primary"
          >
            <h2 className={headingClass}>{title}</h2>
            <ChevronDown className="-rotate-90 shrink-0 text-content-tertiary" aria-hidden />
          </button>
        ) : (
          <h2 className={headingClass}>{title}</h2>
        )}
      </div>
      {isEmpty ? (
        emptyHint !== undefined ? (
          <p className={`${hintClass} px-6 pb-4`}>{emptyHint}</p>
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
 * картинка 128, заголовок и пояснение по центру — «Нет платежей», «Нет
 * просроченных операций» и т.п. — на каноне EmptyState дизайн-слоя. */
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

/** Заголовок над карточками просроченных (вне серой группы). */
export function PaymentsHeading({ children }: { readonly children: ReactNode }): JSX.Element {
  return <h2 className={`${headingClass} px-6`}>{children}</h2>;
}

/** Скелетон секции на время загрузки — серая карточка с пульсирующими
 * строками. */
export function PaymentsSkeleton({ withHeading }: { readonly withHeading?: boolean }): JSX.Element {
  return (
    <section className="mx-6 rounded-card bg-surface-muted px-6 py-6" aria-hidden>
      <Skeleton className={`${withHeading ? 'mb-4' : ''} h-6 w-40 bg-surface-muted-hover`} />
      <div className="flex flex-col gap-4">
        <Skeleton className="h-11 bg-surface-muted-hover" />
        <Skeleton className="h-11 w-4/5 bg-surface-muted-hover" />
      </div>
    </section>
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
 * следующего вхождения или «На паузе» (opacity по истории 22); звезда
 * избранного — ПЕРЕД подзаголовком и во всех состояниях; у платежа с
 * накопленной просрочкой — красная точка-уведомление на иконке
 * (State=Expired, обе поверхности). Поверхность gray — строки на серых
 * блоках (кант серый), white — внутри страницы, открытой кликом по серому
 * блоку (кант белый). Выбор строки открывает страницу платежа (#465). */
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
      : subtitle.kind === 'date'
        ? formatDayMonth(subtitle.iso)
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
