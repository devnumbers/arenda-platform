'use client';

import type { JSX, ReactNode } from 'react';
import { formatDayMonthWithYear } from '@/shared/lib/date-format';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import { rentalDurationLine, rentalTenantTitle } from '@/features/rentals';
import { CategoryIcon, categoryStyle } from '@/features/payment-categories';
import { PropertyAvatar, type PropertyType } from '@/entities/property';
import type { IsoDate } from '@/shared/lib/calendar';
import type { Rental, RentalSummary } from '@/entities/rental';
import { PaymentRowButton } from '@/entities/payment';
import { UserAvatar } from '@/shared/ui/design';

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
  /** Тип объекта строки «Объект» — глиф-плейсхолдер (карта #1217, #1244);
   * не передан — дом-фолбэк. */
  readonly propertyType?: PropertyType;
  /** Фото объекта строки «Объект» (ADR 0065, решение #1286); null — фото
   * нет. */
  readonly propertyPhotoUrl?: string | null;
  /** Blur-кадр фото объекта (ADR 0065 аменд, решение #1594). */
  readonly propertyPhotoBlur?: string | null;
  /** Фото контакта-арендатора из книги (ADR 0065, решение #1286);
   * undefined — контакта нет в книге или у него нет фото. */
  readonly tenantPhotoUrl?: string | undefined;
  /** Blur-кадр фото арендатора (ADR 0065 аменд, решение #1594). */
  readonly tenantPhotoBlur?: string | undefined;
  /** Запись возврата залога: в мастере — из черновика (сумма + комментарий),
   * на итогах завершённой — из записанного; null — записи нет (у аренды без
   * залога пустой комментарий, #1386): секция не выводится вовсе (канон:
   * секция без контента не рисуется). */
  readonly depositReturn: { readonly kopecks: number; readonly comment: string } | null;
};

export function RentalSummaryContent({
  rental,
  summary,
  endDate,
  propertyName,
  propertyType,
  propertyPhotoUrl,
  propertyPhotoBlur,
  tenantPhotoUrl,
  tenantPhotoBlur,
  depositReturn,
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

        {/* Секция выводится только с записью (#1386, канон «секция без
            контента не выводится»); строку «Сумма на возврат» рисует аренда
            с залогом — у завершения без залога запись-ноль («не вернул»)
            смысла суммы не несёт, остаётся комментарий. */}
        {depositReturn !== null && (
          <SummarySection title="Возвращение залога">
            {rental.depositKopecks !== null && (
              <SummaryField
                label="Сумма на возврат"
                value={formatMoneyKopecks(depositReturn.kopecks)}
              />
            )}
            <div className="flex flex-col gap-2">
              <span className="text-base font-medium leading-[18px] text-content">Комментарий</span>
              <div className="min-h-[92px] whitespace-pre-wrap break-words rounded-2xl bg-surface-muted px-[18px] py-[10px] text-base leading-[18px] text-content">
                {depositReturn.comment}
              </div>
            </div>
          </SummarySection>
        )}
      </div>

      {/* Секция по канону Heading (699:8717 — паддинг 24/24/0, макеты
          1433:61927/1433:61074): зазор после «Возвращения залога» 24,
          заголовок несёт свои верх 24 и бока 24, до строк 8. Строки —
          статичные Row Button White без суммы (Figma 1323:60672), круг
          44 и два текста; на всю ширину (на мобайле — экрана, в колонке
          560 — колонки): горизонтали до контента 24 приносит сам ряд
          (12+12), pointer-events-none гасит hover и клики. */}
      <div className="pt-6">
        <h2 className="px-6 pt-6 text-xl font-semibold leading-6 text-content">Данные аренды</h2>
        <div className="flex flex-col pt-2">
          <PaymentRowButton
            variant="white"
            className="pointer-events-none"
            categoryIcon={<CategoryIcon icon={rentStyle.icon} color={rentStyle.color} />}
            title="Арендная плата"
            subtitle="Платеж"
          />
          {tenant !== null && (
            <PaymentRowButton
              variant="white"
              className="pointer-events-none"
              categoryIcon={
                <UserAvatar variant="white" photoUrl={tenantPhotoUrl} photoBlur={tenantPhotoBlur} />
              }
              title={rentalTenantTitle(tenant)}
              subtitle="Арендатор"
            />
          )}
          <PaymentRowButton
            variant="white"
            className="pointer-events-none"
            categoryIcon={
              <PropertyAvatar
                surface="row"
                photoUrl={propertyPhotoUrl ?? null}
                photoBlur={propertyPhotoBlur}
                type={propertyType}
              />
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
