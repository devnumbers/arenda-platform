import type { JSX, ReactNode } from 'react';
import { SmallArrowRight } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';

/**
 * Серая группа-карточка экранов аренды (#531): заголовок «Heading»
 * (Figma 699:8717) — H3 20/24 и SmallArrowRight. Стрелка при тексте —
 * канон #531 (1232:61282); у края карточки — режим edge для экранов
 * «Прошлых аренд» (#535, 1302:52462/1232:61686 — текст растянут, стрелка
 * прижата к правому краю). Карточка — mx-6 (боковые поля экрана), строки
 * и содержимое приносят свои отступы. Хвостовой паддинг карточки и зазор
 * заголовок → содержимое — по макету: «Платеж» 12/8, «Условия аренды»
 * 24/16 (1232:61281), «Арендатор» 16/8 (1232:61565).
 *
 * Кликабельна вся линия заголовка (правило владельца 08.10, #1242 —
 * «область нажатия на блоках должна занимать весь блок»): кнопке заголовка
 * w-full в обоих режимах — и при стрелке у текста (визуал канона #531 не
 * меняется, кликабельным и ховер-зоной становится хвост линии правее
 * стрелки), и в edge.
 */

const headingClass = 'text-xl font-semibold leading-6 text-content';

export function RentalGroup({
  title,
  onOpen,
  openLabel,
  children,
  className,
  contentGap = 'gap-2',
  arrowPosition = 'text',
}: {
  readonly title: string;
  /** Навигация по заголовку; без него заголовок статичен. */
  readonly onOpen?: () => void;
  readonly openLabel?: string;
  readonly children?: ReactNode;
  /** Хвостовой паддинг карточки; дефолт 24 («Условия аренды»). */
  readonly className?: string;
  readonly contentGap?: 'gap-2' | 'gap-4';
  /** Стрелка при тексте (дефолт, #531) или у правого края (#535). */
  readonly arrowPosition?: 'text' | 'edge';
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
            className={cn(
              'flex w-full cursor-pointer items-center gap-3 rounded-pill outline-none transition-opacity hover:opacity-80 active:opacity-80 focus-visible:ring-4 focus-visible:ring-primary',
              arrowPosition === 'edge' ? 'justify-between' : undefined,
            )}
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
