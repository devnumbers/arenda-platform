import type { JSX, ReactNode } from 'react';
import { SmallArrowRight } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';

/**
 * Серая группа-карточка экранов аренды (#531): заголовок «Heading»
 * (Figma 699:8717) — H3 20/24 и SmallArrowRight сразу за текстом
 * (1232:61282 — стрелка навигации при тексте, не у края карточки, как
 * шеврон секций платежей). Карточка — mx-6 (боковые поля экрана), строки
 * и содержимое приносят свои отступы.
 */

const headingClass = 'text-xl font-semibold leading-6 text-content';

export function RentalGroup({
  title,
  onOpen,
  openLabel,
  children,
  className,
}: {
  readonly title: string;
  /** Навигация по заголовку; без него заголовок статичен. */
  readonly onOpen?: () => void;
  readonly openLabel?: string;
  readonly children?: ReactNode;
  readonly className?: string;
}): JSX.Element {
  return (
    <section className={cn('mx-6 rounded-card bg-surface-muted pb-6', className)}>
      <div className="px-6 pb-3 pt-6">
        {onOpen !== undefined ? (
          <button
            type="button"
            onClick={onOpen}
            aria-label={openLabel ?? title}
            className="flex cursor-pointer items-center gap-3 rounded-pill outline-none transition-opacity hover:opacity-80 active:opacity-80 focus-visible:ring-4 focus-visible:ring-primary"
          >
            <h2 className={headingClass}>{title}</h2>
            <SmallArrowRight className="h-6 w-6 shrink-0 text-content" aria-hidden />
          </button>
        ) : (
          <h2 className={headingClass}>{title}</h2>
        )}
      </div>
      {children}
    </section>
  );
}
