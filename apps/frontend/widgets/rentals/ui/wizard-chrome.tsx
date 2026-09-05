import type { ReactNode } from 'react';
import type { JSX } from 'react';
import { cn } from '@/shared/lib/cn';

/**
 * Общий хром шагов визарда создания аренды (#530): заголовок шага —
 * Mobile/Heading/H1 28/32 из макета (1270:46904 — крупнее, чем H3-заголовки
 * визарда платежей), нижняя панель действия над StickyBottomBar.
 */

export function WizardHeading({
  title,
  subtitle,
}: {
  readonly title: string;
  readonly subtitle?: string;
}): JSX.Element {
  return (
    <div className="flex flex-col gap-2 px-6 pt-6">
      <h1 className="m-0 font-sans text-[28px] font-semibold leading-8 text-content">{title}</h1>
      {subtitle !== undefined && (
        <p className="text-sm leading-4 text-content-secondary">{subtitle}</p>
      )}
    </div>
  );
}

export function WizardBottomBar({ children }: { readonly children: ReactNode }): JSX.Element {
  // Без горизонтального паддинга: панель всегда внутри контейнера, где
  // 24px уже есть (StickyBottomBar p-6) — иначе кнопка уже контента.
  return <div className="flex flex-col gap-3">{children}</div>;
}

/** Триггер-бокс поля-пикера (анатомия «Input Field» из макета 1270:46821):
 * заголовок над боксом 56px, значение или плейсхолдер слева, хвостовая
 * иконка справа. Открывает пикер-поверхность снаружи (onOpenChange). */
export function PickerTriggerBox({
  title,
  value,
  placeholder,
  icon,
  onClick,
  error,
  className,
}: {
  readonly title: string;
  /** Значение; undefined — рисуется плейсхолдер приглушённым цветом. */
  readonly value: string | undefined;
  readonly placeholder: string;
  readonly icon: ReactNode;
  readonly onClick: () => void;
  readonly error?: string;
  readonly className?: string;
}): JSX.Element {
  return (
    <div className={cn('flex w-full flex-col gap-2 font-sans', className)}>
      <span className="text-base font-medium leading-[18px] text-content">{title}</span>
      <button
        type="button"
        onClick={onClick}
        aria-label={`${title}: ${value ?? placeholder}`}
        className="flex h-14 w-full cursor-pointer items-center rounded-button bg-surface-muted py-0 pl-[18px] pr-2 text-left transition-shadow outline-none hover:shadow-[inset_0_0_0_2px_var(--dl-input-border)]"
      >
        <span
          className={cn(
            'min-w-0 flex-1 truncate text-base leading-[18px]',
            value === undefined ? 'text-content-secondary' : 'text-content',
          )}
        >
          {value ?? placeholder}
        </span>
        <span aria-hidden className="flex h-11 w-11 shrink-0 items-center justify-center text-content-tertiary">
          {icon}
        </span>
      </button>
      {error !== undefined && <span className="text-[13px] leading-[15px] text-error">{error}</span>}
    </div>
  );
}
