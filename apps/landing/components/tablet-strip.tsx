"use client";

import {
  useEffect,
  useRef,
  useSyncExternalStore,
  type CSSProperties,
  type ReactNode,
} from "react";
import { createStripEngine } from "@/components/strip-engine";
import s from "./tablet-strip.module.css";

// Планшетная полоса карточек (481–1199) на движке лент лендинга
// (components/strip-engine.ts, канон карусели «Управляйте арендой»):
// drag 1:1 за указателем без инерции со снапом на позицию покоя и
// броском, Shift+Scroll — шаг на событие, резиновый край, 360мс
// ease-out (460мс <720). Приходит на смену нативному тач-моментуму
// (решение владельца 28.09 заменено 02.10 — «как у Управляйте арендой»).
//
// Вне планшетного диапазона компонент рендерит прежнюю разметку секции:
// мобайл — стопка, десктоп — статичный ряд; нативный скролл остаётся
// досягаемым до гидратации (useSyncExternalStore даёт серверный снапшот
// false — первый рендер совпадает с SSR, переключение после маунта,
// без рассинхрона). Геометрию обеих разметок задаёт секция классами;
// нативная полоса на планшете — утилита strip-scroll (globals.css).
const MEDIA = "(min-width: 481px) and (max-width: 1199px)";

function subscribe(onChange: () => void) {
  const mq = window.matchMedia(MEDIA);
  mq.addEventListener("change", onChange);
  return () => {
    mq.removeEventListener("change", onChange);
  };
}

export function TabletStrip({
  className,
  nativeClassName,
  viewportClassName,
  activeChildren,
  centerWhenFit = false,
  stripWidth = 984,
  children,
}: {
  // Внешний контейнер — на всех ярусах (отступы секции).
  className?: string;
  // Классы полосы вне планшетного диапазона — прежняя разметка секции
  // (мобайл-стопка, нативный планшетный скролл до гидратации, десктоп-ряд).
  nativeClassName?: string;
  // Доп. классы окна движка (full-bleed полосы тарифов: -mx-6 и ширина).
  viewportClassName?: string;
  // Дети активного режима, если они отличаются от нативной разметки:
  // карточки напрямую детьми дорожки — движок меряет шаг и длину стрипа
  // по прямым детям, группирующий контейнер (сетка тарифов) сломал бы
  // позиции покоя по карточкам.
  activeChildren?: ReactNode;
  // Когда стрип влезает (1032–1199), центрировать его паддингом дорожки —
  // эквивалент нативного w-fit + mx-auto (CardTrio/Audience). Тарифам не
  // нужно: их полоса живёт в контейнере секции и не центрируется.
  centerWhenFit?: boolean;
  // Ширина стрипа для центрирования: 3×320 + 2 зазора по 12.
  stripWidth?: number;
  children: ReactNode;
}) {
  const active = useSyncExternalStore(
    subscribe,
    () => window.matchMedia(MEDIA).matches,
    () => false,
  );
  const viewportRef = useRef<HTMLDivElement>(null);
  const trackRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!active) {
      return;
    }
    const vp = viewportRef.current;
    const track = trackRef.current;
    if (!vp || !track) {
      return;
    }
    const engine = createStripEngine(vp, track, {
      // Геометрия планшетных полос — поле 24px с обоих краёв.
      tier: () => ({ gap: 12, column: (w) => w - 48 }),
      draggingClass: s.dragging,
      // Мышь ленту не перетаскивает (слово владельца 02.10 — «мышкой не
      // тянуть»); скролл жив: трекпад/Shift+Scroll шагают ленту, тач
      // тянет (слово владельца 05.10 — «на планшете скролится как было»).
      // На ПК (≥1200) движка нет вовсе — ряды неподвижны.
      noMouseDrag: true,
    });
    return () => {
      engine.destroy();
    };
  }, [active]);

  if (!active) {
    return (
      <div className={`${className ?? ""} ${nativeClassName ?? ""}`.trim()}>
        {children}
      </div>
    );
  }

  return (
    // w-full только в активном режиме: окно движка с negative-маржинами
    // (полоса тарифов) считает ширину от полного контейнера; в нативном
    // режиме классы секции не тронуты.
    <div className={`${className ?? ""} w-full`.trim()}>
      <div
        ref={viewportRef}
        className={`${s.viewport} ${centerWhenFit ? s.centerFit : ""} ${viewportClassName ?? ""}`.trim()}
      >
        <div
          ref={trackRef}
          className={s.track}
          style={{ "--strip-w": `${stripWidth}px` } as CSSProperties}
        >
          {activeChildren ?? children}
        </div>
      </div>
    </div>
  );
}
