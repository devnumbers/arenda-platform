import type { JSX } from 'react';
import {
  Skeleton,
  SkeletonFormField,
  SkeletonListRow,
  SkeletonRoundAction,
  skeletonBlockClass,
  skeletonRowWidths,
} from '@/shared/ui/design';

/**
 * Скелетоны экранов аренды (паритет #604, §7 DESIGN.md): каждый зеркалит
 * анатомию своего экрана — детализация активной аренды (#606), шаг 1
 * визарда (#607), карточка условий (#531), список прошлых аренд (#535),
 * завершённая детализация (#535), итоги (#535), форма правки условий
 * (#532) и продление (#533). Шапки экранов рендерятся вне фазы загрузки
 * и скелетоном не подменяются; обёртки скелетонов повторяют обёртки
 * контента (зазоры и верхний отступ — как у живой страницы), чтобы данные
 * занимали место скелетона без сдвига.
 */

/** Приглушённый тон блоков внутри серых карточек (§7, skeletonBlockClass). */
const MUTED = skeletonBlockClass('muted');

/** Заголовок секции-заглушки: каркас RentalGroup — H3 20/24 и стрелка. */
function SkeletonGroupHeader({ titleWidth, arrow = true }: {
  readonly titleWidth: string;
  readonly arrow?: boolean;
}): JSX.Element {
  return (
    <span className="flex items-center gap-3">
      <Skeleton className={`h-6 ${titleWidth} ${MUTED}`} />
      {arrow && <Skeleton className={`h-6 w-6 shrink-0 ${MUTED}`} />}
    </span>
  );
}

/** Заголовок серой группы у края карточки (#535): текст слева, стрелка
 * прижата к правому краю (RentalGroup, arrowPosition="edge"). */
function SkeletonGroupHeaderEdge({ titleWidth }: { readonly titleWidth: string }): JSX.Element {
  return (
    <span className="flex w-full items-center justify-between">
      <Skeleton className={`h-6 ${titleWidth} ${MUTED}`} />
      <Skeleton className={`h-6 w-6 shrink-0 ${MUTED}`} />
    </span>
  );
}

/** Строки условий «метка — значение» (TermRow): сетка из двух равных
 * колонок с зазором 12, высота строки 16. Ширины — детерминированный
 * цикл skeletonRowWidths. Боковой отступ приносит потребитель: живой
 * TermRows несёт свой px-6, а внутри p-6-карточки условий строкам
 * паддинг не нужен. */
function SkeletonTermRows({ rows }: { readonly rows: number }): JSX.Element {
  const widths = skeletonRowWidths(rows);
  return (
    <div className="flex flex-col gap-2">
      {widths.map((width, index) => (
        <span key={index} className="grid grid-cols-2 gap-3">
          <Skeleton className={`h-4 ${width.title} ${MUTED}`} />
          <Skeleton className={`h-4 ${width.subtitle} ${MUTED}`} />
        </span>
      ))}
    </div>
  );
}

export function RentalDetailSkeleton(): JSX.Element {
  return (
    <div aria-hidden className="flex flex-col gap-12">
      {/* Картинка-ключ 96 (1232:61491) и «Оплачено N из M». */}
      <div className="flex flex-col items-center gap-4">
        <Skeleton className="h-24 w-24" />
        <div className="flex flex-col items-center gap-2">
          <Skeleton className="h-[18px] w-20" />
          <Skeleton className="h-8 w-3/5" />
        </div>
      </div>

      {/* Круглая тройка макета (1550:93664, решение #802): «Завершить /
          Продлить / Оплатить» равными колонками. */}
      <div className="grid grid-cols-3 justify-items-center">
        <SkeletonRoundAction />
        <SkeletonRoundAction />
        <SkeletonRoundAction />
      </div>

      <div className="flex flex-col gap-4">
        {/* «Платеж»: строка правила p-3 на серой карточке (pb-3). */}
        <section className="mx-6 flex flex-col gap-2 rounded-card bg-surface-muted pb-3">
          <div className="px-6 pt-6">
            <SkeletonGroupHeader titleWidth="w-24" />
          </div>
          <SkeletonListRow value tone="muted" className="px-3 py-3" />
        </section>

        {/* Прогресс: строка следующего платежа, бар, строка остатка. */}
        <section className="mx-6 rounded-card bg-surface-muted p-6">
          <div className="flex flex-col gap-5">
            <div className="flex flex-col gap-3">
              <Skeleton className={`h-4 w-2/5 ${MUTED}`} />
              <Skeleton className={`h-1.5 w-full rounded-pill ${MUTED}`} />
            </div>
            <Skeleton className={`h-4 w-1/3 ${MUTED}`} />
          </div>
        </section>

        {/* «Условия аренды»: 3 строки «метка — значение» (зазор секции 16). */}
        <section className="mx-6 flex flex-col gap-4 rounded-card bg-surface-muted pb-6">
          <div className="px-6 pt-6">
            <SkeletonGroupHeader titleWidth="w-40" />
          </div>
          <div className="px-6">
            <SkeletonTermRows rows={3} />
          </div>
        </section>

        {/* «Арендатор»: строка канона px-3 py-2 (pb-4). Секция выводится
            только с арендатором (тело #531) — скелетон моделирует каноничную
            композицию с арендатором; у аренды без него секции не будет. */}
        <section className="mx-6 flex flex-col gap-2 rounded-card bg-surface-muted pb-4">
          <div className="px-6 pt-6">
            <SkeletonGroupHeader titleWidth="w-28" />
          </div>
          <SkeletonListRow tone="muted" className="px-3 py-2" />
        </section>

        {/* «Управление»: три строки ListRow (иконка 24 + заголовок) —
            «Редактировать / Завершить / Продлить» (#532/#534/#533), без
            стрелки. */}
        <section className="mx-6 flex flex-col gap-2 rounded-card bg-surface-muted pb-3">
          <div className="px-6 pt-6">
            <SkeletonGroupHeader titleWidth="w-32" arrow={false} />
          </div>
          <div className="flex flex-col">
            {Array.from({ length: 3 }, (_, index) => (
              <span key={index} className="flex items-center gap-3 px-6 py-2">
                <Skeleton className={`h-6 w-6 shrink-0 ${MUTED}`} />
                <Skeleton className={`h-6 w-1/2 ${MUTED}`} />
              </span>
            ))}
          </div>
        </section>
      </div>
    </div>
  );
}

/** Скелетон карточки условий read-only (#531): одна серая карточка p-6 —
 * восемь строк «метка — значение» и блок комментария под ними; общая для
 * текущей и архивной («В архиве») вариаций экрана. */
export function RentalTermsCardSkeleton(): JSX.Element {
  return (
    <div aria-hidden className="px-6">
      <section className="flex flex-col gap-6 rounded-card bg-surface-muted p-6">
        <SkeletonTermRows rows={8} />
        <div className="flex flex-col gap-1">
          <Skeleton className={`h-4 w-24 ${MUTED}`} />
          <Skeleton className={`h-4 w-2/5 ${MUTED}`} />
        </div>
      </section>
    </div>
  );
}

/** Скелетон списка «Прошлых аренд» (#535): карточки RentalGroup со
 * стрелкой у края — заголовок-срок, три строки условий teaser; строка
 * арендатора в скелетон не входит (появляется только с арендатором).
 * Обёртка повторяет контентную (зазор 16, верхний отступ 8). */
export function RentalPastListSkeleton(): JSX.Element {
  return (
    <div aria-hidden className="flex flex-col gap-4 pt-2">
      {Array.from({ length: 2 }, (_, index) => (
        <section key={index} className="mx-6 flex flex-col rounded-card bg-surface-muted pb-4">
          <div className="px-6 pt-6 pb-4">
            <SkeletonGroupHeaderEdge titleWidth="w-32" />
          </div>
          <div className="px-6">
            <SkeletonTermRows rows={3} />
          </div>
        </section>
      ))}
    </div>
  );
}

/** Скелетон завершённой детализации (#535): герой (картинка-ключ 96,
 * заголовок-срок, который ждёт список операций, и круглая заглушка кнопки
 * итогов) и четыре серые группы — «Условия аренды» (3 teaser-строки),
 * «История операций» (пара строк), «Арендатор» (строка канона),
 * «Управление» (строка удаления владельца). Секционные ожидания операций
 * остаются внутри живой страницы. */
export function RentalCompletedSkeleton(): JSX.Element {
  return (
    <div aria-hidden className="flex flex-col gap-12">
      <div className="flex flex-col items-center gap-4">
        <Skeleton className="h-24 w-24" />
        <Skeleton className="h-8 w-56" />
        <Skeleton className="h-14 w-14 rounded-pill" />
      </div>

      <div className="flex flex-col gap-4">
        {/* «Условия аренды»: стрелка у края, 3 teaser-строки (contentGap 16). */}
        <section className="mx-6 flex flex-col gap-4 rounded-card bg-surface-muted pb-6">
          <div className="px-6 pt-6">
            <SkeletonGroupHeaderEdge titleWidth="w-40" />
          </div>
          <div className="px-6">
            <SkeletonTermRows rows={3} />
          </div>
        </section>

        {/* «История операций»: пара строк высоты операции (pb-3). */}
        <section className="mx-6 flex flex-col gap-2 rounded-card bg-surface-muted pb-3">
          <div className="px-6 pt-6">
            <SkeletonGroupHeaderEdge titleWidth="w-44" />
          </div>
          <div className="flex flex-col gap-4 px-6 pb-2">
            <Skeleton className={`h-11 w-full ${MUTED}`} />
            <Skeleton className={`h-11 w-4/5 ${MUTED}`} />
          </div>
        </section>

        {/* «Арендатор»: строка канона px-3 py-2 (pb-4). */}
        <section className="mx-6 flex flex-col gap-2 rounded-card bg-surface-muted pb-4">
          <div className="px-6 pt-6">
            <SkeletonGroupHeaderEdge titleWidth="w-28" />
          </div>
          <SkeletonListRow tone="muted" className="px-3 py-2" />
        </section>

        {/* «Управление» (только владельцу): строка удаления, без стрелки. */}
        <section className="mx-6 flex flex-col gap-2 rounded-card bg-surface-muted pb-3">
          <div className="px-6 pt-6">
            <SkeletonGroupHeader titleWidth="w-32" arrow={false} />
          </div>
          <div className="flex items-center gap-3 px-6 py-2">
            <Skeleton className={`h-6 w-6 shrink-0 ${MUTED}`} />
            <Skeleton className={`h-6 w-36 ${MUTED}`} />
          </div>
        </section>
      </div>
    </div>
  );
}

/** Секция итогов: заголовок H2 20/24 и пары «метка — поле» (поле h-14,
 * радиус 2xl — как поле summary); поля — прямые дети секции, зазор 24
 * (SummarySection: плоский flex gap-6). Тон блоков базовый: секции лежат
 * на белом (§7 — muted только внутри серых карточек). */
function SkeletonSummarySection({ titleWidth, fields }: {
  readonly titleWidth: string;
  readonly fields: number;
}): JSX.Element {
  return (
    <section className="flex flex-col gap-6">
      <Skeleton className={`h-6 ${titleWidth}`} />
      {Array.from({ length: fields }, (_, index) => (
        <span key={index} className="flex flex-col gap-2">
          <Skeleton className="h-[18px] w-28" />
          <Skeleton className="h-14 w-full rounded-2xl" />
        </span>
      ))}
    </section>
  );
}

/** Скелетон «Итогов аренды» (#535): секции «Период аренды» (3 поля),
 * «Финансы за время аренды» (3 поля), «Возвращение залога» (поле и блок
 * комментария) — обёртка контента (зазор 48, боковые 24), ниже «Данные
 * аренды» (три строки канона с ведущим кругом) с отступом 64. Тон блоков
 * базовый: секции лежат на белом (§7 — muted только внутри серых
 * карточек). */
export function RentalSummarySkeleton(): JSX.Element {
  return (
    <div aria-hidden className="pb-6">
      <div className="flex flex-col gap-12 px-6">
        <SkeletonSummarySection titleWidth="w-40" fields={3} />
        <SkeletonSummarySection titleWidth="w-56" fields={3} />
        <section className="flex flex-col gap-6">
          <Skeleton className="h-6 w-48" />
          <span className="flex flex-col gap-2">
            <Skeleton className="h-[18px] w-32" />
            <Skeleton className="h-14 w-full rounded-2xl" />
          </span>
          <div className="flex flex-col gap-2">
            <Skeleton className="h-[18px] w-28" />
            <Skeleton className="h-[92px] w-full rounded-2xl" />
          </div>
        </section>
      </div>
      <div className="pt-16">
        <div className="px-6">
          <Skeleton className="h-6 w-36" />
        </div>
        <div className="flex flex-col px-3 pt-2">
          <SkeletonListRow leading className="px-3 py-2" />
          <SkeletonListRow leading className="px-3 py-2" />
          <SkeletonListRow leading className="px-3 py-2" />
        </div>
      </div>
    </div>
  );
}

/** Скелетон формы правки условий (#532): семь полевых строк канона
 * (MoneyField, пикеры, статичное начало, деньги, пикер коммунальных
 * платежей) — метка 16/18 и серый бокс h-14, обёртка шага (зазор 32).
 * Тумблер автоплатежа, блок комментария и панель сохранения — ниже
 * сгиба, панель появляется с формой. */
export function RentalTermsEditFormSkeleton(): JSX.Element {
  return (
    <div aria-hidden className="flex flex-col gap-8 px-6 pt-6">
      {Array.from({ length: 7 }, (_, index) => (
        <SkeletonFormField key={index} labelWidth={index % 2 === 0 ? 'w-28' : 'w-24'} />
      ))}
    </div>
  );
}

/** Скелетон продления (#533): заголовок «На сколько продлить аренду?»
 * (H1 20/24) с подзаголовком и поле «Новая дата» — обёртка шага
 * (зазор 32). Панель «Отменить / Продлить» появляется с формой. */
export function RentalExtendSkeleton(): JSX.Element {
  return (
    <div aria-hidden className="flex flex-col gap-8 px-6 pt-6">
      <div className="flex flex-col gap-2">
        <Skeleton className="h-6 w-64" />
        <Skeleton className="h-[18px] w-1/2" />
      </div>
      <SkeletonFormField labelWidth="w-24" />
    </div>
  );
}

/** Скелетон ленты «Истории операций» завершённой аренды (#535): группы по
 * датам — заголовок H2 и строки канона с порядковым номером (subtitle) и
 * суммой (value), как HistoryRow; общий для ожидания аренды и ожидания
 * ленты. Чип сортировки скелетоном не подменяется — живой чип рендерится
 * вне фазы загрузки, над скелетоном (§7). */
export function RentalHistoryFeedSkeleton(): JSX.Element {
  const widths = skeletonRowWidths(4);
  return (
    <div aria-hidden className="flex flex-col">
      {Array.from({ length: 2 }, (_, group) => (
        <section key={group} className="flex flex-col">
          <div className="px-6">
            <Skeleton className="h-6 w-24" />
          </div>
          <div className="flex flex-col">
            {widths.slice(group * 2, group * 2 + 2).map((width, index) => (
              <SkeletonListRow key={index} subtitle value widths={width} className="px-3 py-3" />
            ))}
          </div>
        </section>
      ))}
    </div>
  );
}

/** Скелетон шага подтверждения мастера завершения (#534): герой «Завершить
 * аренду?» — ключ 96, заголовок H1 и две строки описания (обёртка шага:
 * зазор 32, боковые 48). Панель «Отменить / Продолжить» появляется с
 * формой. */
export function RentalCompleteConfirmSkeleton(): JSX.Element {
  return (
    <div aria-hidden className="flex flex-col items-center gap-8 px-12 pt-6">
      <Skeleton className="h-24 w-24" />
      <div className="flex flex-col items-center gap-2">
        <Skeleton className="h-6 w-44" />
        <Skeleton className="h-[18px] w-3/4" />
        <Skeleton className="h-[18px] w-1/2" />
      </div>
    </div>
  );
}

/**
 * Скелетон шага 1 визарда создания аренды «Цена и число оплаты» (#607,
 * паритет — §7 DESIGN.md): каркас AmountDayStep — MoneyField «Арендная
 * плата» и PickerTriggerBox «День оплаты» (те же вставки шага: pt-6,
 * зазор 8). Заголовок шага и хром (крестик, чип шага) рендерятся вне
 * фазы загрузки; кнопка шага скрыта до готовности — в загрузке её нет.
 */
export function RentalAmountDayStepSkeleton(): JSX.Element {
  return (
    <div aria-hidden className="flex flex-col gap-8 px-6 pt-6">
      <SkeletonFormField labelWidth="w-28" />
      <SkeletonFormField labelWidth="w-24" />
    </div>
  );
}
