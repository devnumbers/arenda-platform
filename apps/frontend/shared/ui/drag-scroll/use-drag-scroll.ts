"use client";

import {
  useCallback,
  useEffect,
  useRef,
  type DragEvent,
  type PointerEvent as ReactPointerEvent,
  type RefObject,
} from "react";
import { animate, type AnimationPlaybackControls } from "framer-motion";
import { useReducedMotion } from "@/shared/lib/hooks/useReducedMotion";
import { createVelocityTracker } from "@/shared/ui/pull-to-refresh/pull-math";
import type { VelocityTracker } from "@/shared/ui/pull-to-refresh/pull-math";
import {
  exponentialRide,
  isHorizontalDrag,
  pickSnapTarget,
  projectStopPoint,
} from "./drag-scroll-math";

/**
 * Mouse-drag прокси поверх нативного горизонтального snap-скролла
 * (тикет #1349; research/apple-style-drag-hub-sections.md — вариант А).
 *
 * Контракт: перетаскивание ленты мышью/пером с инерцией «как у Apple» —
 * `scrollLeft` следует за указателем; на отпускании цель считается заранее
 * чистым модулем (проекция остановки + ближайшая snap-точка — веб-аналог
 * `targetContentOffset`) и доезжается экспоненциальным tween `animate()`
 * (exponentialRide). Тач не заходит в код
 * вовсе (гвард `pointerType`), поэтому нативный momentum/bounce и
 * `scroll-snap-type: mandatory` тач-лент не трогаются. На время жеста snap
 * выключается штатным off-switch (`scroll-snap-type: none`) и
 * восстанавливается строго после остановки доезда — раньше нельзя, браузер
 * перещёлкнул бы рывком. Клик по карточке, начавшийся как drag ≤ порога,
 * живёт; после реального drag первый click гасится (capture-фаза), иначе
 * отпускание на карточке открыло бы платёж.
 *
 * Края без резинки: нативный скроллер не даёт scrollLeft за границы,
 * десктоп-браузеры батутят сами (macOS) — имитация не отличима от натива
 * и не нужна (research §2.2/§5). RTL ленты не поддерживаются — жест
 * отключается (приложение LTR; нормализация scrollLeft RTL — при
 * локализации, research §7.5).
 */

export type DragScrollControls = {
  /** Реф на скроллер-ленту. */
  readonly ref: RefObject<HTMLDivElement | null>;
  /** Pointer-обработчики разметки ленты. */
  readonly onPointerDown: (event: ReactPointerEvent<HTMLDivElement>) => void;
  /** Нативный drag картинок/ссылок крадёт указатель (pointercancel) —
   * отменяется; жест pointer-событиями продолжается. */
  readonly onDragStart: (event: DragEvent<HTMLDivElement>) => void;
};

type Gesture = {
  readonly pointerId: number;
  readonly startX: number;
  readonly startY: number;
  readonly startScrollLeft: number;
  dragging: boolean;
  readonly tracker: VelocityTracker;
};

/** Snap-позиции детей ленты: `snap-start` выравнивает левый край карточки
 * на линию `scroll-padding-left` (bleed ≤560 держит 24px), поэтому позиция
 * снапа = левый край ребёнка относительно нуля скролла минус паддинг.
 * Точки клампятся в достижимый диапазон scrollLeft: хвост ленты за пределами
 * максимальной прокрутки снапится к концу (нативное состояние конца ленты —
 * последняя карточка вплотную к краю), иначе цель сильного флика уводится
 * за границу и доезд упирается в кламп мимо снапа. */
function collectSnapPoints(element: HTMLDivElement): number[] {
  const railRect = element.getBoundingClientRect();
  const style = getComputedStyle(element);
  const padding = Number.parseFloat(style.scrollPaddingLeft) || 0;
  const zeroAtViewportX = railRect.left - element.scrollLeft;
  const maxScrollLeft = element.scrollWidth - element.clientWidth;
  return Array.from(element.children).map((child) => {
    const point = child.getBoundingClientRect().left - zeroAtViewportX - padding;
    return Math.min(Math.max(point, 0), maxScrollLeft);
  });
}

export function useDragScroll(): DragScrollControls {
  // Ручная мемоизация (стандарт React Compiler): колбэки живут в нативных
  // window-слушателях, навешиваемых на pointerdown, — пересоздание замыканий
  // между рендерами осиротило бы слушатели идущего жеста (прецедент PTR).
  const ref = useRef<HTMLDivElement | null>(null);
  const gestureRef = useRef<Gesture | null>(null);
  const rideRef = useRef<AnimationPlaybackControls | null>(null);
  const cleanupRef = useRef<() => void>(() => {});
  const reducedMotion = useReducedMotion();
  const reducedMotionRef = useRef(reducedMotion);
  useEffect(() => {
    reducedMotionRef.current = reducedMotion;
  }, [reducedMotion]);

  // Off-switch snap'а на время жеста и возврат после доезда; курсор здесь
  // же — один реставратор на все inline-стили жеста.
  const restoreSnap = useCallback((element: HTMLDivElement): void => {
    element.style.scrollSnapType = "";
    element.style.scrollBehavior = "";
    element.style.cursor = "";
  }, []);

  const stopRide = useCallback((): void => {
    rideRef.current?.stop();
    rideRef.current = null;
  }, []);

  /** Доезд к snap-цели. Цель — чистая математика модуля: проекция
   * остановки по скорости отпускания (power·velocity) и ближайшая к ней
   * snap-точка — веб-аналог targetContentOffset. Кривая доезда —
   * экспоненциальное затухание (exponentialRide), длительность производная
   * от дистанции и timeConstant (не фиксированные 350ms --dl-ease: дальний
   * флик на фиксе дёрганый — research §5). */
  const rideToSnap = useCallback(
    (element: HTMLDivElement, velocityPxPerS: number): void => {
      const snapPoints = collectSnapPoints(element);
      if (snapPoints.length === 0) {
        restoreSnap(element);
        return;
      }
      const ideal = projectStopPoint(element.scrollLeft, velocityPxPerS);
      const target = pickSnapTarget(ideal, snapPoints);
      if (target === null) {
        restoreSnap(element);
        return;
      }
      if (reducedMotionRef.current) {
        // Дизайн-канон §8: жест — управление (не гасится), доезд — движение
        // (схлопывается в мгновенную установку целевой точки).
        element.scrollLeft = target;
        restoreSnap(element);
        return;
      }
      const { durationMs, ease } = exponentialRide(
        element.scrollLeft,
        target,
      );
      if (durationMs === 0) {
        element.scrollLeft = target;
        restoreSnap(element);
        return;
      }
      rideRef.current = animate(element.scrollLeft, target, {
        duration: durationMs / 1000,
        ease,
        onUpdate: (latest) => {
          element.scrollLeft = latest;
        },
        onComplete: () => {
          rideRef.current = null;
          restoreSnap(element);
        },
      });
    },
    [restoreSnap],
  );

  /** Первый click после реального drag гасится (capture-фаза), затем
   * слушатель снимается. Клик dispatch-ится синхронно после pointerup, до
   * таймеров — нулевой таймер разоружает слушатель, если отпускание было
   * мимо карточек и клика не будет вовсе. */
  const suppressNextClick = useCallback((element: HTMLDivElement): void => {
    const swallow = (clickEvent: Event): void => {
      clickEvent.stopPropagation();
      clickEvent.preventDefault();
      element.removeEventListener("click", swallow, true);
    };
    element.addEventListener("click", swallow, true);
    window.setTimeout(() => element.removeEventListener("click", swallow, true), 0);
  }, []);

  const onPointerDown = useCallback(
    (event: ReactPointerEvent<HTMLDivElement>): void => {
      const element = ref.current;
      if (element === null) return;
      if (event.pointerType !== "mouse" && event.pointerType !== "pen") return;
      if (event.button !== 0) return;
      if (getComputedStyle(element).direction === "rtl") return;
      // Новый жест главнее бегущего доезда (как в PTR) — гасим; snap ещё
      // выключен, на pointerup вернём, даже если до порога дело не дошло.
      stopRide();

      const gesture: Gesture = {
        pointerId: event.pointerId,
        startX: event.clientX,
        startY: event.clientY,
        startScrollLeft: element.scrollLeft,
        dragging: false,
        tracker: createVelocityTracker(),
      };
      gesture.tracker.add(event.timeStamp, element.scrollLeft);
      gestureRef.current = gesture;

      const onMove = (move: globalThis.PointerEvent): void => {
        const current = gestureRef.current;
        const rail = ref.current;
        if (current === null || rail === null || move.pointerId !== current.pointerId) {
          return;
        }
        const dx = move.clientX - current.startX;
        const dy = move.clientY - current.startY;
        if (!current.dragging) {
          if (!isHorizontalDrag(dx, dy)) return;
          current.dragging = true;
          rail.style.scrollSnapType = "none";
          rail.style.scrollBehavior = "auto";
          rail.style.cursor = "grabbing";
          try {
            // Capture гасит hover-мигание карточек под курсором; click после
            // capture целится в ленту — но он и так гасится после drag.
            rail.setPointerCapture(current.pointerId);
          } catch {
            // Указатель уже ушёл — жест доживёт без capture.
          }
        }
        // Совместимые mousemove гасятся — выделение текста под drag не едет.
        move.preventDefault();
        rail.scrollLeft = current.startScrollLeft - dx;
        current.tracker.add(move.timeStamp, rail.scrollLeft);
      };

      const finish = (velocityPxPerS: number, ride: boolean): void => {
        const rail = ref.current;
        cleanupRef.current();
        gestureRef.current = null;
        if (rail === null) return;
        // Украденный жест (pointercancel): мгновенно вернуть snap, браузер
        // беззвучно встанет в ближайшую точку.
        if (!ride) {
          restoreSnap(rail);
          return;
        }
        // Клик (порог не взят) поверх остановленного доезда — не рывком, а
        // коротким микродоездом к ближайшей точке (velocity 0).
        if (!gesture.dragging) {
          rideToSnap(rail, 0);
          return;
        }
        suppressNextClick(rail);
        rideToSnap(rail, velocityPxPerS);
      };

      const onUp = (up: globalThis.PointerEvent): void => {
        const current = gestureRef.current;
        if (current === null || up.pointerId !== current.pointerId) return;
        finish(current.tracker.velocityPxPerS(), true);
      };

      // Указатель забрал браузер (native drag, жест ОС) — без доезда:
      // velocity уже не про наш контент, мгновенно вернуть snap.
      const onCancel = (cancel: globalThis.PointerEvent): void => {
        const current = gestureRef.current;
        if (current === null || cancel.pointerId !== current.pointerId) return;
        finish(0, false);
      };

      cleanupRef.current();
      window.addEventListener("pointermove", onMove);
      window.addEventListener("pointerup", onUp);
      window.addEventListener("pointercancel", onCancel);
      cleanupRef.current = () => {
        window.removeEventListener("pointermove", onMove);
        window.removeEventListener("pointerup", onUp);
        window.removeEventListener("pointercancel", onCancel);
        cleanupRef.current = () => {};
      };
    },
    [restoreSnap, rideToSnap, stopRide, suppressNextClick],
  );

  const onDragStart = useCallback((event: DragEvent<HTMLDivElement>): void => {
    event.preventDefault();
  }, []);

  // Колесо/тачпад поверх бегущего доезда: гасим анимацию и возвращаем snap —
  // нативный скролл колеса сам встанет в ближайшую точку (research §7.7).
  useEffect(() => {
    const element = ref.current;
    if (element === null) return;
    const onWheel = (): void => {
      if (rideRef.current === null) return;
      stopRide();
      restoreSnap(element);
    };
    element.addEventListener("wheel", onWheel, { passive: true });
    return () => {
      element.removeEventListener("wheel", onWheel);
      cleanupRef.current();
      stopRide();
    };
  }, [restoreSnap, stopRide]);

  return { ref, onPointerDown, onDragStart };
}
