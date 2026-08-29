import type { ReactNode } from 'react';
import type { JSX } from 'react';

/**
 * Общий хром шагов визарда создания платежа (#464): заголовок шага
 * (Figma Heading 699:8717 — H3 20/24 + подзаголовок 14/16), нижняя
 * панель действия над StickyBottomBar и подсказка открытого поиска
 * (Figma 1049:46256 — иллюстрация 128 + текст 16/18).
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

/** Контент шага при открытом поиске категорий: вместо списка — иллюстрация
 * с одним текстом (пустой запрос — «Начните искать», без совпадений —
 * «Ничего не нашлось»); создание категории здесь не упоминается. */
export function CategorySearchHint({ text }: { readonly text: string }): JSX.Element {
  return (
    <div className="flex flex-col items-center gap-4 pt-16">
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        src="/images/payments/category-search.png"
        alt=""
        width={128}
        height={128}
        className="h-32 w-32"
      />
      <p className="max-w-[320px] text-center text-base leading-[18px] text-content-secondary">
        {text}
      </p>
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
