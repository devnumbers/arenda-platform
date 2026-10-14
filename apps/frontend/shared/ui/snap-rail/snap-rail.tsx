"use client";

import useEmblaCarousel from "embla-carousel-react";
import { WheelGesturesPlugin } from "embla-carousel-wheel-gestures";
import type { JSX } from "react";
import type { ReactNode } from "react";
import { useMemo } from "react";
import { cn } from "@/shared/lib/cn";
import { useReducedMotion } from "@/shared/lib/hooks/useReducedMotion";
import { snapRailOptions } from "./snap-rail-options";

/** Required CSS Embla (docs/guides/required-setup): вьюпорт прячет вылет
 * (overflow: hidden — обязательно), контейнер — flex с axis-lock для тача
 * (вертикальный свайп по ленте крутит страницу). */
const VIEWPORT_CLASSES = "overflow-hidden";
const CONTAINER_CLASSES = "flex gap-2 [touch-action:pan-y_pinch-zoom]";

/**
 * Горизонтальная snap-лента на Embla v9 (тикет #1349, решение владельца
 * 13.10 — переход с кастомного pointer-прокси на Embla dragFree: 'snap').
 * Канон механики — DESIGN.md §8 «Drag горизонтальных snap-лент»: единый
 * движок мыши и тача, инерция с доездом к ближайшему снапу, резинка у
 * краёв, колесо/трекпад через wheel-gestures, reduced-motion без
 * инерционного доезда. Дети — карточки фиксированной ширины со shrink-0;
 * допклассы потребителя (bleed ≤560: `max-tablet:-mx-6 max-tablet:px-6`)
 * ложатся на вьюпорт поверх канонных, линия снапа = левый край
 * контейнера (он сидит внутри bleed-паддинга). Вживую: ленты секций хаба
 * «Платежей» (widgets/payments/ui/payments-global-screen.tsx).
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
  const reducedMotion = useReducedMotion();
  // Ручная мемоизация: опции уходят в не-React недра Embla, а смена
  // идентичности — это reInit; пресобирать объект каждый рендер нельзя.
  const options = useMemo(
    () => snapRailOptions({ reducedMotion }),
    [reducedMotion],
  );
  // Свой экземпляр плагина на ленту: состояние плагина живёт в замыкании.
  const plugins = useMemo(() => [WheelGesturesPlugin()], []);
  const [emblaRef] = useEmblaCarousel(options, plugins);

  return (
    <div
      ref={emblaRef}
      data-testid={testId}
      className={cn(VIEWPORT_CLASSES, className)}
    >
      <div className={CONTAINER_CLASSES}>{children}</div>
    </div>
  );
}
