"use client";

import type { JSX } from "react";
import type { ReactNode } from "react";
import { cn } from "@/shared/lib/cn";
import { useSnapRailDrag } from "./use-snap-rail-drag";

/** Канонные классы горизонтальной snap-ленты: нативный overflow-скролл со
 * скрытым скроллбаром, snap-proximity (тач/колесо браузера; решение
 * владельца 18.10 — «слишком резкий»: proximity даёт свободный скролл с
 * мягким дотягиванием, mandatory с жёстким дожатием после каждого флика
 * снят; iOS трактует proximity как mandatory — на iPhone поведение прежнее,
 * drag-прокси мыши сажает ленту точно на снап-линию своим доездом).
 * touch-action `pan-x pan-y pinch-zoom`: pan-x РАЗРЕШАЕТ браузеру вести
 * горизонтальную ось (нативный скролл ленты на таче работает, только если
 * ось разрешена touch-action — без него свайп пальцем по ленте мёртв, баг
 * 18.10: тут стоял одиночный pan-y по образцу JS-карусели, где
 * горизонталь ведёт свой JS), pan-y держит вертикальный свайп за
 * страницей; выбор оси — первичная ось жеста, браузер разбирает сам.
 * Дети — snap-start ([&>*]: карточкам не нужно помнить канон): без
 * snap-align у детей браузерный снап мёртв. */
const RAIL_CLASSES =
  "flex snap-x snap-proximity gap-2 overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden [&>*]:snap-start [touch-action:pan-x_pan-y_pinch-zoom]";

/** Классы ширины fluid-плитки ленты (решение владельца 18.10 — «2 на
 * мобилке, 3 на планшете/ПК»): проценты резолвятся по контент-боксу ленты —
 * с bleed ≤560 это вьюпорт минус 2×24, выше — колонка минус 2×24. Единый
 * источник для плитки `PaymentCardButton`, её скелетона и витрин — сетка
 * 24/8 не расползается. */
export const SNAP_TILE_CLASSES =
  "w-[calc((100%-8px)/2)] tablet:w-[calc((100%-16px)/3)]";

/**
 * Горизонтальная snap-лента (#1349; решение владельца 14.10 — возврат с
 * Embla на своё решение): нативный скролл — тач, колесо и перо ведёт
 * браузер (momentum, резинка и снап UIScrollView на iOS — оригинал, не
 * имитация); мышь/перо тащат ленту drag-прокси с инерцией к ближайшей
 * snap-точке проекции остановки и резинкой у краёв (useSnapRailDrag,
 * физика Apple — snap-rail-physics). Дети — snap-start карточки
 * fluid-ширины со shrink-0 (плитка растягивается: 2 карточки на
 * мобайл-ярусе, 3 на планшете/ПК — `w-[calc((100%-8px)/2)]
 * tablet:w-[calc((100%-16px)/3)]` у PaymentCardButton, решение владельца
 * 18.10); допклассы потребителя (bleed ≤560:
 * `max-tablet:-mx-6 max-tablet:px-6 max-tablet:[scroll-padding-inline:
 * 24px]`) ложатся на скроллер поверх канонных — scroll-padding и есть
 * линия снапа. Вживую: ленты секций хаба «Платежей»
 * (widgets/payments/ui/payments-global-screen.tsx), витрина /ui-kit.
 */
export function SnapRail({
  testId,
  className,
  children,
}: {
  readonly testId?: string;
  readonly className?: string;
  readonly children: ReactNode;
}): JSX.Element {
  const { railRef, contentRef, onPointerDown, onDragStart } = useSnapRailDrag();

  return (
    <div
      ref={railRef}
      onPointerDown={onPointerDown}
      onDragStart={onDragStart}
      data-testid={testId}
      className={cn(RAIL_CLASSES, className)}
    >
      {/* w-full обязателен: контент-див — флекс-айтем с auto-шириной
       * (max-content) — без явной ширины проценты fluid-плиток резолвятся
       * циклически и мусорят геометрию снапа. w-full = контент-бокс ленты,
       * против него и считают плитки. */}
      <div ref={contentRef} className="flex w-full gap-2">
        {children}
      </div>
    </div>
  );
}
