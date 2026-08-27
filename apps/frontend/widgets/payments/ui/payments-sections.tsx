import type { JSX, ReactNode } from 'react';
import { ChevronDown, Star } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';
import {
  formatDayMonth,
  formatOverdueDays,
  PaymentRowButton,
} from '@/entities/payment';
import type { IsoDate, Payment, PaymentOperation } from '@/entities/payment';
import { CategoryIcon, categoryStyle } from '@/features/payment-categories';
import { daysOverdue } from '../lib/overdue-days';
import { paymentRowSubtitle } from '../lib/payment-row-subtitle';

/**
 * Секции экрана «Платежи объекта» (Figma 654:6778, 853:17208): серые
 * группы-карточки с заголовком секции, пустые состояния с подсказкой,
 * скелетоны загрузки и строка платежа (иконка категории с белым кантом на
 * серой группе, подзаголовок — дата следующего вхождения или «На паузе»,
 * звезда избранного). Стрелки-ссылки заголовков не рисуются — адресаты в
 * следующих срезах.
 */

const headingClass = 'text-xl font-semibold leading-6 text-content';
const hintClass = 'text-[13px] leading-[15px] text-content-secondary';

/** Серая группа секции со строками (заголовок 24/24 сверху, строки приносят
 * свои 24px горизонтали); боковые поля 24 — поля экрана из Figma. Со
 * стрелкой-навигацией в заголовке (стрелки секций страницы платежа —
 * резолюция #452): на «Платежах объекта» стрелки не рисуются — адресаты
 * других срезов. */
export function PaymentsGroup({
  title,
  open,
  children,
}: {
  readonly title: string;
  readonly open?: { readonly label: string; readonly onOpen: () => void };
  readonly children: ReactNode;
}): JSX.Element {
  return (
    <section className="mx-6 rounded-card bg-surface-muted pb-2">
      <div className="flex items-center justify-between px-6 pb-3 pt-6">
        <h2 className={headingClass}>{title}</h2>
        {open !== undefined && (
          <button
            type="button"
            onClick={open.onOpen}
            aria-label={open.label}
            className="-m-1 cursor-pointer rounded-pill p-1 text-content-tertiary outline-none transition-opacity hover:opacity-80 active:opacity-80 focus-visible:ring-4 focus-visible:ring-primary"
          >
            <ChevronDown className="-rotate-90" aria-hidden />
          </button>
        )}
      </div>
      {children}
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
      <div className={`${withHeading ? 'mb-4' : ''} h-6 w-40 animate-pulse rounded-pill bg-surface-muted-hover`} />
      <div className="flex flex-col gap-4">
        <div className="h-11 animate-pulse rounded-pill bg-surface-muted-hover" />
        <div className="h-11 w-4/5 animate-pulse rounded-pill bg-surface-muted-hover" />
      </div>
    </section>
  );
}

/** Просроченная операция (страница платежа #465 и полный список #466,
 * фреймы 693:5435/850:15412): срок «N дней» и сумма красным, бейдж danger
 * на иконке категории. Порядок asc — старейшая первой, долг разбирают по
 * порядку накопления. Поверхность: gray — секция страницы, white — строки
 * «Графика» в полном списке (резолюция #452). */
export function OverdueOperationRow({
  operation,
  today,
  variant = 'gray',
  className,
}: {
  readonly operation: PaymentOperation;
  readonly today: IsoDate;
  readonly variant?: 'white' | 'gray';
  readonly className?: string;
}): JSX.Element {
  const style = categoryStyle('default', operation.categorySlug);

  return (
    <PaymentRowButton
      className={cn('px-3', className)}
      variant={variant}
      danger
      categoryIcon={
        <CategoryIcon icon={style.icon} color={style.color} badge="danger" surface="muted" />
      }
      title={operation.title}
      description={formatOverdueDays(daysOverdue(operation.date, today))}
      amountKopecks={operation.amountKopecks}
    />
  );
}

/** Строка платежа в серой группе: подзаголовок — дата следующего вхождения
 * или «На паузе» (opacity по истории 22), звезда избранного после даты
 * (Figma 654:6778). Выбор строки открывает страницу платежа (#465). */
export function PaymentRow({
  payment,
  today,
  onSelect,
}: {
  readonly payment: Payment;
  readonly today: IsoDate;
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
      variant="gray"
      className={paused ? 'px-3 opacity-60' : 'px-3'}
      categoryIcon={<CategoryIcon icon={style.icon} color={style.color} surface="white" />}
      title={payment.title}
      subtitle={
        dateText !== undefined && payment.isFavorite ? (
          <span className="inline-flex items-center gap-1">
            {dateText}
            <Star className="h-4 w-4 shrink-0" aria-hidden />
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
