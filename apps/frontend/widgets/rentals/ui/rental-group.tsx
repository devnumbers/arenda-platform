import type { JSX, ReactNode } from 'react';
import { SmallArrowRight } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';

/**
 * Серая группа-карточка экранов аренды (#531): заголовок «Heading»
 * (Figma 699:8717) — H3 20/24 и SmallArrowRight у правого края (решение
 * владельца 08.10, #1242: «как на странице объекта»; прежний канон #531
 * «стрелка при тексте» (1232:61282) для групп аренды отменён — для экранов
 * «Прошлых аренд» (#535, 1302:52462/1232:61686) стрелка у края и была
 * каноном, режимы слились). Карточка — mx-6 (боковые поля экрана), строки
 * и содержимое приносят свои отступы. Хвостовой паддинг карточки и зазор
 * заголовок → содержимое — по макету: «Платеж» 12/8, «Условия аренды»
 * 24/16 (1232:61281), «Арендатор» 16/8 (1232:61565).
 *
 * Кликабельна вся линия заголовка (правило владельца 08.10, #1242 —
 * «область нажатия на блоках должна занимать весь блок»): кнопка w-full
 * justify-between — заголовок слева, стрелка у правого края; ховер-зона —
 * вся линия; клавиатура/role — нативная кнопка.
 */

const headingClass = 'text-xl font-semibold leading-6 text-content';

export function RentalGroup({
  title,
  onOpen,
  openLabel,
  children,
  className,
  contentGap = 'gap-2',
}: {
  readonly title: string;
  /** Навигация по заголовку; без него заголовок статичен. */
  readonly onOpen?: () => void;
  readonly openLabel?: string;
  readonly children?: ReactNode;
  /** Хвостовой паддинг карточки; дефолт 24 («Условия аренды»). */
  readonly className?: string;
  readonly contentGap?: 'gap-2' | 'gap-4';
}): JSX.Element {
  const arrow = <SmallArrowRight className="h-6 w-6 shrink-0 text-content" aria-hidden />;
  return (
    <section className={cn('mx-6 flex flex-col rounded-card bg-surface-muted pb-6', contentGap, className)}>
      <div className="px-6 pt-6">
        {onOpen !== undefined ? (
          <button
            type="button"
            onClick={onOpen}
            aria-label={openLabel ?? title}
            className="flex w-full cursor-pointer items-center justify-between gap-3 rounded-pill outline-none transition-opacity hover:opacity-80 active:opacity-80 focus-visible:ring-4 focus-visible:ring-primary"
          >
            <h2 className={headingClass}>{title}</h2>
            {arrow}
          </button>
        ) : (
          <h2 className={headingClass}>{title}</h2>
        )}
      </div>
      {children}
    </section>
  );
}
