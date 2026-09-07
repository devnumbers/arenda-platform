'use client';

import type { JSX, ReactNode } from 'react';
import { BoldHome, BoldUser } from '@/shared/assets/icons';
import { formatDayMonthWithYear } from '@/shared/lib/date-format';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import { rentalDurationLine, rentalTenantTitle } from '@/features/rentals';
import { CategoryIcon, categoryStyle } from '@/features/payment-categories';
import type { IsoDate } from '@/shared/lib/calendar';
import type { Rental, RentalSummary } from '@/entities/rental';
import { PaymentRowButton } from '@/entities/payment';

/**
 * Содержание экрана «Подведем итоги аренды» (#534, Figma 1433:61927 —
 * десктоп, 1433:61389 — планшет, 1433:61074 — мобильный обзор): секции
 * «Период аренды», «Финансы за время аренды» и «Возвращение залога» —
 * read-only значения в серых боксах анатомии «Input Field», «Данные
 * аренды» — строки Row Button. Финансы — все paid-операции объекта за
 * [начало, завершение] (решение №13); отрицательная прибыль — с минусом
 * в начале. Хром (шапка шага, кнопки, mobile-hero) — у потребителей;
 * компонент переиспользуется материалом завершённой аренды (#535).
 */

export type RentalSummaryContentProps = {
  readonly rental: Rental;
  readonly summary: RentalSummary;
  /** Дата окончания периода итогов: выбранная в мастере либо завершения. */
  readonly endDate: IsoDate;
  readonly propertyName: string;
  /** Записанный возврат залога — сумма из черновика мастера (копейки). */
  readonly depositReturnKopecks: number;
  /** Комментарий возврата из черновика мастера. */
  readonly depositReturnComment: string;
};

export function RentalSummaryContent({
  rental,
  summary,
  endDate,
  propertyName,
  depositReturnKopecks,
  depositReturnComment,
}: RentalSummaryContentProps): JSX.Element {
  const tenant = rental.tenant;
  const rentStyle = categoryStyle('default', 'rent');

  return (
    <div className="pb-6">
      <div className="flex flex-col gap-12 px-6">
        <SummarySection title="Период аренды">
          <SummaryField label="Срок аренды" value={rentalDurationLine(rental.startDate, endDate)} />
          <SummaryField
            label="Начало аренды"
            value={formatDayMonthWithYear(rental.startDate, rental.today)}
          />
          <SummaryField label="Окончание аренды" value={formatDayMonthWithYear(endDate, rental.today)} />
        </SummarySection>

        <SummarySection title="Финансы за время аренды">
          <SummaryField label="Прибыль" value={formatMoneyKopecks(summary.profitKopecks)} />
          <SummaryField label="Доходы" value={formatMoneyKopecks(summary.incomeKopecks)} />
          <SummaryField label="Расходы" value={formatMoneyKopecks(summary.expenseKopecks)} />
        </SummarySection>

        <SummarySection title="Возвращение залога">
          <SummaryField
            label="Сумма на возврат"
            value={formatMoneyKopecks(depositReturnKopecks)}
          />
          <div className="flex flex-col gap-2">
            <span className="text-base font-medium leading-[18px] text-content">Комментарий</span>
            <div className="min-h-[92px] whitespace-pre-wrap break-words rounded-2xl bg-surface-muted px-[18px] py-[10px] text-base leading-[18px] text-content">
              {depositReturnComment}
            </div>
          </div>
        </SummarySection>
      </div>

      <div className="pt-16">
        <h2 className="px-6 text-xl font-semibold leading-6 text-content">Данные аренды</h2>
        <div className="flex flex-col px-3 pt-2">
          <DataRow
            categoryIcon={<CategoryIcon icon={rentStyle.icon} color={rentStyle.color} />}
            title="Арендная плата"
            subtitle="Платеж"
          />
          {tenant !== null && (
            <DataRow
              categoryIcon={
                <span
                  aria-hidden
                  className="flex h-11 w-11 items-center justify-center rounded-pill bg-surface-muted"
                >
                  <BoldUser className="h-6 w-6 text-content" />
                </span>
              }
              title={rentalTenantTitle(tenant)}
              subtitle="Арендатор"
            />
          )}
          <DataRow
            categoryIcon={
              <span
                aria-hidden
                className="flex h-11 w-11 items-center justify-center rounded-pill bg-surface-muted"
              >
                <BoldHome className="h-6 w-6 text-content" />
              </span>
            }
            title={propertyName}
            subtitle="Объект"
          />
        </div>
      </div>
    </div>
  );
}

/** Секция полей (Figma 1433:62435): заголовок Heading 20/24, между полями
 * и заголовком — 24. */
function SummarySection({
  title,
  children,
}: {
  readonly title: string;
  readonly children: ReactNode;
}): JSX.Element {
  return (
    <section className="flex flex-col gap-6">
      <h2 className="text-xl font-semibold leading-6 text-content">{title}</h2>
      {children}
    </section>
  );
}

/** Read-only поле итогов (анатомия «Input Field» 1218:53539): заголовок
 * 16/18 medium над боксом 56. */
function SummaryField({ label, value }: { readonly label: string; readonly value: string }): JSX.Element {
  return (
    <div className="flex flex-col gap-2">
      <span className="text-base font-medium leading-[18px] text-content">{label}</span>
      <div className="flex h-14 items-center rounded-2xl bg-surface-muted px-[18px] text-base leading-[18px] text-content">
        {value}
      </div>
    </div>
  );
}

/** Строка «Данных аренды» (Row Button White, Figma 1323:60672 без суммы):
 * круг-иконка 44, название и серый подзаголовок; статичная, без действия. */
function DataRow({
  categoryIcon,
  title,
  subtitle,
}: {
  readonly categoryIcon: ReactNode;
  readonly title: string;
  readonly subtitle: string;
}): JSX.Element {
  return (
    <PaymentRowButton
      variant="white"
      className="pointer-events-none px-3"
      categoryIcon={categoryIcon}
      title={title}
      subtitle={subtitle}
    />
  );
}
