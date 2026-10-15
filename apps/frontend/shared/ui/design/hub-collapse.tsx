'use client';

import { useEffect, useRef, type JSX, type ReactNode } from 'react';

/** Высота бара хедера (без safe-area) — с ней сравнивается позиция якоря. */
const BAR_HEIGHT = 72;

/** Группа хаб-шапки, сворачивающаяся в компакт-бар при прокрутке
 * (решение владельца 2026-09-09, Figma 1603-93157 → 1733-93740):
 * оборачивает заголовок раздела с пилюлей поиска/чипами и пишет прогресс
 * сворачивания (0..1) прямыми style-записями в потребителей
 * .hub-compact/.hub-compact-bar TopNav, минуя React-состояние и
 * CSS-переменные (аудит #1418, план 011: переменная на :root каждый кадр
 * инвалидирует унаследованные custom properties по всему дереву; rest-
 * значения потребителей — globals.css). Прогресс = доля скролла, за которую
 * группа проходит от стартовой позиции до полного ухода под бар; до 1
 * доводится, когда нижний край группы доходит до кромки бара. Только
 * opacity/transform у потребителей, без reflow и ре-рендеров на кадр
 * (rAF-троттлинг, passive-listener). prefers-reduced-motion — порог
 * вместо непрерывного прогресса. На странице один хаб — один якорь. */
export function HubCollapseAnchor({ children }: { readonly children: ReactNode }): JSX.Element {
  const anchorRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const anchor = anchorRef.current;
    if (!anchor) return;
    const root = document.documentElement;
    const reduced = window.matchMedia('(prefers-reduced-motion: reduce)');

    let frame = 0;
    let last = -1;

    const update = () => {
      frame = 0;
      const rect = anchor.getBoundingClientRect();
      const restTop = rect.top + window.scrollY;
      const range = restTop + rect.height - BAR_HEIGHT;
      let progress = range > 0 ? window.scrollY / range : 1;
      progress = Math.min(1, Math.max(0, progress));
      if (reduced.matches) progress = progress > 0.5 ? 1 : 0;
      if (Math.abs(progress - last) < 0.004) return;
      last = progress;
      // Прогресс — прямыми записями в потребителей (аудит #1418, канон
      // перфа: никаких custom properties на :root каждый кадр —
      // инвалидирует всё дерево). Потребителей двое, оба в TopNav.
      for (const el of document.querySelectorAll<HTMLElement>('.hub-compact')) {
        el.style.opacity = String(progress);
      }
      for (const el of document.querySelectorAll<HTMLElement>('.hub-compact-bar')) {
        // Точность 3 знака как у прежней записи переменной; Math.round вместо
        // toFixed — гейт #330 держит toFixed в форматных модулях.
        el.style.transform = `translateY(${Math.round((1 - progress) * -100000) / 1000}%)`;
      }
      root.classList.toggle('hub-collapse-on', progress > 0.04);
    };

    const request = () => {
      if (!frame) frame = requestAnimationFrame(update);
    };
    const invalidate = () => {
      last = -1;
      request();
    };

    update();
    window.addEventListener('scroll', request, { passive: true });
    window.addEventListener('resize', invalidate);
    return () => {
      window.removeEventListener('scroll', request);
      window.removeEventListener('resize', invalidate);
      if (frame) cancelAnimationFrame(frame);
      root.classList.remove('hub-collapse-on');
      for (const el of document.querySelectorAll<HTMLElement>('.hub-compact, .hub-compact-bar')) {
        el.style.removeProperty('opacity');
        el.style.removeProperty('transform');
      }
    };
  }, []);

  return <div ref={anchorRef}>{children}</div>;
}
