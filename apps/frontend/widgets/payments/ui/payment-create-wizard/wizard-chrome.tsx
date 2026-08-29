import type { ReactNode } from 'react';
import type { JSX } from 'react';

/**
 * Общий хром шагов визарда создания платежа (#464): заголовок шага
 * (Figma Heading 699:8717 — H3 20/24 + подзаголовок 14/16) и нижняя
 * панель действия над StickyBottomBar.
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
      <h1 className="text-xl font-semibold leading-6 text-content">{title}</h1>
      {subtitle !== undefined && (
        <p className="text-sm leading-4 text-content-secondary">{subtitle}</p>
      )}
    </div>
  );
}

export function WizardBottomBar({ children }: { readonly children: ReactNode }): JSX.Element {
  // Без горизонтального паддинга: панель всегда внутри контейнера, где
  // 24px уже есть (StickyBottomBar p-6, экран успеха px-6) — иначе кнопка
  // уже контента (двойные 48px). Без нижнего тоже: у StickyBottomBar свой
  // 24px + safe-area.
  return <div className="flex flex-col gap-3">{children}</div>;
}
