'use client';

import { animate } from 'framer-motion';
import { useEffect, useRef, type JSX, type ReactNode } from 'react';

/** Высота бара хедера (без safe-area) — с ней сравнивается позиция якоря. */
const BAR_HEIGHT = 72;
/** Магнит (решение владельца 2026-09-09, «средний»): зона доводки и
 * параметры пружин. Зона 0.2–0.8 — гистерезис против дрожания на границе;
 * доводка ~220ms с мягким допружиниванием (bounce 0.15). Слежение —
 * плотная пружина с лёгкой инерцией. */
const SETTLE_ZONE = 0.2;
const FOLLOW_SPRING = { type: 'spring', stiffness: 300, damping: 34 } as const;
const SETTLE_SPRING = { type: 'spring', duration: 0.22, bounce: 0.15 } as const;
/** Фолбэк idle-детекта для браузеров без scrollend (см. caniuse). */
const IDLE_TIMEOUT = 160;

/** Группа хаб-шапки, сворачивающаяся в компакт-бар при прокрутке
 * (решение владельца 2026-09-09, Figma 1603:93157 → 1733:93740):
 * оборачивает заголовок раздела с пилюлей поиска/чипами и пишет прогресс
 * сворачивания в CSS-переменную `--hub-collapse` (0..1) на :root — её
 * потребляют `hub-compact`-блоки TopNav (см. globals.css). Прогресс = доля
 * скролла, за которую группа проходит от стартовой позиции до полного
 * ухода под бар.
 *
 * «Магнит» (решение владельца 2026-09-09, средний): движение идёт через
 * пружину (компакт слегка отстаёт от пальца и допружинивает), а когда
 * скролл затих (`scrollend`, в браузерах без него — idle-таймаут) и
 * прогресс остался в зоне 0.2–0.8, доводится до ближайшего конца. Значение
 * анимируется через `animate()` framer-motion (зависимость уже была) и
 * пишется напрямую в style корня, минуя React-состояние — только
 * opacity/transform у потребителей, без reflow и ре-рендеров на кадр.
 * prefers-reduced-motion — мгновенный порог вместо пружин. На странице
 * один хаб — один якорь. */
export function HubCollapseAnchor({ children }: { readonly children: ReactNode }): JSX.Element {
  const anchorRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const anchor = anchorRef.current;
    if (!anchor) return;
    const root = document.documentElement;
    const reduced = window.matchMedia('(prefers-reduced-motion: reduce)');

    let displayed = 0;
    let controls: { stop: () => void } | null = null;
    let frame = 0;
    let idleTimer = 0;

    const write = (value: number) => {
      displayed = value;
      root.style.setProperty('--hub-collapse', String(Math.round(value * 1000) / 1000));
      root.classList.toggle('hub-collapse-on', value > 0.04);
    };

    const targetProgress = () => {
      const rect = anchor.getBoundingClientRect();
      const restTop = rect.top + window.scrollY;
      const range = restTop + rect.height - BAR_HEIGHT;
      const progress = range > 0 ? window.scrollY / range : 1;
      return Math.min(1, Math.max(0, progress));
    };

    const follow = (target: number) => {
      controls?.stop();
      if (reduced.matches) {
        write(target > 0.5 ? 1 : 0);
        return;
      }
      controls = animate(displayed, target, { ...FOLLOW_SPRING, onUpdate: write });
    };

    const settle = () => {
      const target = targetProgress();
      if (target <= SETTLE_ZONE || target >= 1 - SETTLE_ZONE) return;
      const end = target > 0.5 ? 1 : 0;
      controls?.stop();
      controls = reduced.matches
        ? null
        : animate(displayed, end, { ...SETTLE_SPRING, onUpdate: write });
      if (reduced.matches) write(end);
    };

    const hasScrollend = typeof window.onscrollend === 'function';

    const onScroll = () => {
      if (!frame) frame = requestAnimationFrame(() => { frame = 0; follow(targetProgress()); });
      if (!hasScrollend) {
        window.clearTimeout(idleTimer);
        idleTimer = window.setTimeout(settle, IDLE_TIMEOUT);
      }
    };
    const invalidate = () => follow(targetProgress());

    follow(targetProgress());
    window.addEventListener('scroll', onScroll, { passive: true });
    window.addEventListener('resize', invalidate);
    if (hasScrollend) window.addEventListener('scrollend', settle);
    return () => {
      window.removeEventListener('scroll', onScroll);
      window.removeEventListener('resize', invalidate);
      if (hasScrollend) window.removeEventListener('scrollend', settle);
      window.clearTimeout(idleTimer);
      if (frame) cancelAnimationFrame(frame);
      controls?.stop();
      root.style.removeProperty('--hub-collapse');
      root.classList.remove('hub-collapse-on');
    };
  }, []);

  return <div ref={anchorRef}>{children}</div>;
}
