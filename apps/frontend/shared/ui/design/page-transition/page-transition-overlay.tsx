'use client';

import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { type JSX, useEffect, useSyncExternalStore } from 'react';
import { registerPageTransitionHandler } from '@/shared/lib/navigation';
import {
  extractTransitionAnchor,
  isModifiedClick,
  resolveTransitionHref,
} from './page-transition-click';
import { createPageTransitionMachine } from './page-transition-machine';

/** Машина шторки — модульный синглтон: оверлей рисует его фазу, канал
 * навигации (pushPage/replacePage/goBack из shared/lib/navigation) и
 * перехват кликов заводят в него переходы. */
const machine = createPageTransitionMachine();

/* Лист — Tailwind-утилиты на токенах (канон design-слоя без CSS-модулей):
 * база — transition-opacity на --dl-duration-move, покрытие короче —
 * --dl-duration-fast, reduced-motion укорачивает до 150ms (§8); значения
 * обязаны совпадать с константами page-transition-machine.ts — страж
 * дрейфа сверяет их с tokens.css в page-transition-overlay.test.ts. */
const OVERLAY_CLASS = [
  'fixed inset-0 z-[2147483000] bg-surface opacity-0 pointer-events-none',
  'transition-opacity duration-(--dl-duration-move) ease-apple will-change-[opacity]',
  'data-[phase=covering]:opacity-100 data-[phase=covering]:pointer-events-auto data-[phase=covering]:duration-(--dl-duration-fast)',
  'data-[phase=revealing]:opacity-0 data-[phase=revealing]:pointer-events-auto',
  'motion-reduce:duration-150 motion-reduce:data-[phase=covering]:duration-150',
].join(' ');

/**
 * Шторка плавных переходов между страницами (порт PageTransitionOverlay
 * alterix-new): клик по same-origin ссылке или программный переход
 * (pushPage/replacePage/goBack) заливает экран листом цвета поверхности
 * (--dl-surface), под ним едет навигация, по смене URL лист уходит.
 * Решение о клике — resolveTransitionHref; у шторки нет своего UI,
 * кроме полноэкранного aria-hidden листа. Тайминги канона DESIGN.md §8,
 * reduced-motion укорачивает до ~150ms. Исключение для якоря —
 * [data-page-transition-skip]. Браузерные Back/Forward без шторки:
 * popstate нельзя перехватить до навигации.
 */
export function PageTransitionOverlay(): JSX.Element {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const phase = useSyncExternalStore(machine.subscribe, machine.getPhase);
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
