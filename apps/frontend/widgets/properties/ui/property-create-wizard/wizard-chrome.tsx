import type { JSX, ReactNode } from 'react';

/**
 * Общий хром шагов визарда создания объекта (#480): заголовок шага
 * (H1 28/32 — Figma Mobile/Heading/H1/600, шаги 1 и 3) с необязательной
 * подсказкой 16/18 #9FA8AC (шаг 3, Figma 1218:54295) и панель действия
 * над StickyBottomBar.
 */

export function PropertyWizardHeading({
  title,
  hint,
}: {
  readonly title: string;
  readonly hint?: string;
}): JSX.Element {
  return (
    <div className="flex flex-col gap-3 px-6 pt-6">
      <h1 className="m-0 text-2xl font-semibold text-content">{title}</h1>
      {hint !== undefined && (
        <p className="m-0 text-base leading-[18px] text-content-tertiary">{hint}</p>
      )}
    </div>
  );
}

/** Панель действия шага. Без горизонтального паддинга: панель всегда
 * внутри контейнера, где 24px уже есть (StickyBottomBar p-6) — иначе
 * кнопка уже контента (двойные 48px). Без нижнего тоже: у
 * StickyBottomBar свой 24px + safe-area. */
export function PropertyWizardBottomBar({ children }: { readonly children: ReactNode }): JSX.Element {
  return <div className="flex flex-col gap-3">{children}</div>;
}
