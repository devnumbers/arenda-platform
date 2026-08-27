import type { JSX, ReactNode } from 'react';
import { Star } from '@/shared/assets/icons';
import { formatDayMonth, PaymentRowButton } from '@/entities/payment';
import type { IsoDate, Payment } from '@/entities/payment';
import { CategoryIcon, categoryStyle } from '@/features/payment-categories';
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
 * свои 24px горизонтали); боковые поля 24 — поля экрана из Figma. */
export function PaymentsGroup({
  title,
  children,
}: {
  readonly title: string;
  readonly children: ReactNode;
}): JSX.Element {
  return (
    <section className="mx-6 rounded-card bg-surface-muted pb-2">
      <h2 className={`${headingClass} px-6 pb-3 pt-6`}>{title}</h2>
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
