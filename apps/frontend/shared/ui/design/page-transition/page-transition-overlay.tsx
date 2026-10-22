'use client';

import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { type JSX, useEffect, useSyncExternalStore } from 'react';
import { registerPageTransitionHandler } from '@/shared/lib/navigation';
import {
  extractTransitionAnchor,
  isModifiedClick,
  resolveTransitionHref,
} from './page-transition-click';
import { createPageTransitionMachine, type TransitionPhase } from './page-transition-machine';

/** Машина переходов — модульный синглтон: оверлей рисует её фазу, канал
 * навигации (pushPage/replacePage/goBack из shared/lib/navigation) и
 * перехват кликов заводят в неё переходы. Раннер view transitions —
 * единственное DOM-касание машины: есть document.startViewTransition —
 * основной путь кроссфейда, нет (Safari < 18, Firefox < 144) — шторка.
 * Гвард typeof — страница рендерится и на сервере, module-level
 * document там запрещён. */
const machine = createPageTransitionMachine(
  typeof document === 'undefined' || typeof document.startViewTransition !== 'function'
    ? undefined
    : (update) => {
        const transition = document.startViewTransition(update);
        return { skip: () => transition.skipTransition() };
      },
);

/* Лист — Tailwind-утилиты на токенах (канон design-слоя без CSS-модулей):
 * база — transition-opacity на --pt-reveal-duration, покрытие короче —
 * --pt-cover-duration, reduced-motion укорачивает до 100ms (override в
 * tokens.css); значения обязаны совпадать с константами
 * page-transition-machine.ts — страж дрейфа сверяет их с tokens.css в
 * page-transition-overlay.test.ts. */
const OVERLAY_CLASS = [
  'fixed inset-0 z-[2147483000] bg-surface opacity-0 pointer-events-none',
  'transition-opacity duration-(--pt-reveal-duration) ease-apple will-change-[opacity]',
  'data-[phase=covering]:opacity-100 data-[phase=covering]:pointer-events-auto data-[phase=covering]:duration-(--pt-cover-duration)',
  'data-[phase=revealing]:opacity-0 data-[phase=revealing]:pointer-events-auto',
  'motion-reduce:duration-100 motion-reduce:data-[phase=covering]:duration-100',
].join(' ');

/**
 * Переходы между страницами (канон DESIGN.md §8, решения владельца
 * 21.10). Основной движок — кроссфейд View Transitions API: по тапу
 * машина стартует document.startViewTransition, старый экран остаётся
 * живым (фаза vt-pending шторку не зажигает), навигация едет внутри
 * update-колбэка, по смене URL браузер сам анимирует пару old/new одним
 * кроссфейдом --pt-crossfade-duration (правила ::view-transition-* — в
 * globals.css). Шторка ниже — фолбэк браузеров без VT и предохранитель
 * медленной сети (watchdog 600ms машины накрывает экран листом,
 * кроссфейд скипается). Решение о клике — resolveTransitionHref; у
 * оверлея нет своего UI, кроме полноэкранного aria-hidden листа
 * фолбэка. Исключение для якоря — [data-page-transition-skip].
 * Браузерные Back/Forward без анимации: popstate нельзя перехватить до
 * навигации.
 */
export function PageTransitionOverlay(): JSX.Element {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const phase = useSyncExternalStore(
    machine.subscribe,
    machine.getPhase,
    (): TransitionPhase => 'idle',
  );
  const urlKey = `${pathname}?${searchParams.toString()}`;

  // Канал программной навигации живёт, пока оверлей смонтирован: в
  // логине/лендинге обработчика нет — навигация прямая. reset при
  // размонтировании гасит таймеры, чтобы лист не остался висеть.
  useEffect(() => {
    registerPageTransitionHandler((perform) => {
      machine.navigate(perform);
    });
    return () => {
      registerPageTransitionHandler(null);
      machine.reset();
    };
  }, []);

  useEffect(() => {
    machine.notifyNavigated();
  }, [urlKey]);

  useEffect(() => {
    const handleClick = (event: MouseEvent): void => {
      const href = resolveTransitionHref(
        {
          defaultPrevented: event.defaultPrevented,
          button: event.button,
          modified: isModifiedClick(event),
        },
        extractTransitionAnchor(event),
        window.location.href,
      );

      if (href === null) {
        return;
      }

      event.preventDefault();
      router.prefetch(href);
      machine.navigate(() => {
        router.push(href);
      });
    };

    const handlePageShow = (): void => {
      machine.reset();
    };

    document.addEventListener('click', handleClick, { capture: true });
    window.addEventListener('pageshow', handlePageShow);

    return () => {
      document.removeEventListener('click', handleClick, { capture: true });
      window.removeEventListener('pageshow', handlePageShow);
    };
  }, [router]);

  return <div className={OVERLAY_CLASS} data-phase={phase} aria-hidden="true" />;
}
