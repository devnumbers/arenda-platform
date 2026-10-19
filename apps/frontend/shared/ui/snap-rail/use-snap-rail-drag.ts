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
import {
  createVelocityTracker,
  rubberBandOffset,
} from "@/shared/ui/pull-to-refresh/pull-math";
import type { VelocityTracker } from "@/shared/ui/pull-to-refresh/pull-math";
import {
  criticalSettle,
  isHorizontalDrag,
  pickSnapTarget,
  projectStopPoint,
} from "./snap-rail-physics";

/** Зеркало кривой дома --dl-ease (tokens.css) для reduced-motion-доката:
 * JS-аниматор не читает var(), дубликат вынужденный (прецедент LiveValue). */
const DL_EASE: [number, number, number, number] = [0.32, 0.72, 0, 1];
/** Reduced-motion: жест — управление (не гасится), докат к ближайшему снапу
 * укорачивается до 150мс (канон §8 «укорачиваем, не отключаем»). Единица —
 * секунды (format `animate` motion). */
const REDUCED_SETTLE_S = 0.15;

/**
 * Drag-прокси мыши/пера поверх нативного горизонтального snap-скролла
 * (#1349; решение владельца 14.10 — возврат с Embla, физика по эталону
 * Apple; канон — DESIGN.md §8).
 *
 * Контракт: `scrollLeft` ленты следует за указателем 1:1; скорость
 * считается по RAW-позиции (без клампа краёв — скорость живёт и в вязнущей
 * резинке); на отпускании цель проецируется заранее (projectStopPoint,
 * d=0.998 — веб-аналог `scrollViewWillEndDragging`) и доезжается критическим
 * демпфированием без перелёта со стартом от скорости отпускания
 * (criticalSettle: velocity handoff, адаптивная ω ≥ |v|/|d| — доезд
 * стартует с пальца без «кирпичной стены» на коротких фликах,
 * apple-design §5; решение владельца 18.10 — «слишком резкий»).
 *
 * Резинка у краёв: за правый край диапазон расширяется настоящим полем
 * контента (scrollLeft уходит за обычный максимум нативно, гэп после
 * карточек и есть резинка), за левый — трансформом контента (в LTR левое
 * переполнение в диапазон не входит); на отпускании поле гасится доездом
 * scrollLeft, трансформ — параллельной пружиной с лёгким отскоком
 * (damping ~0.8 — таблица Apple для momentum-жестов; канон §8 держит
 * отскок только в резинке) и velocity handoff — край «липнет» к пальцу и
 * пружинит обратно, как UIScrollView (решение владельца 18.10 — «не
 * липкий у края»). Velocity считается по RAW-позиции (без клампа) —
 * живёт и в вязнущей резинке. У тача резинка нативная — тач в код не
 * заходит вовсе (гвард `pointerType`), momentum/bounce/снап браузера
 * (`snap-proximity` канона — см. snap-rail.tsx) целы.
 *
 * Сохранённые механики 94ba68fec (проверены живой приёмкой 12/12): snap
 * выключается на время жеста и восстанавливается строго после доезда;
 * клик живёт до порога, после реального drag первый click гасится
 * capture-фазой; pointercancel (native drag, жест ОС) — мгновенный возврат
 * snap; колесо/новый pointerdown гасят бегущий доезд; RTL ленты жестом не
 * обслуживаются (приложение LTR). Выделение текста реальным вводом гасится
 * на активном жесте: user-select:none на ленте + очистка начавшегося до
 * порога, снятие реставратором после жеста (#1436; preventDefault
 * совместимых mousemove drag-select в Chromium не останавливает).
 */

export type SnapRailDragControls = {
  /** Реф на скроллер-ленту (нативный overflow-скролл). */
  readonly railRef: RefObject<HTMLDivElement | null>;
  /** Реф на flex-контейнер детей — на нём играет резинка-трансформ. */
  readonly contentRef: RefObject<HTMLDivElement | null>;
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
  /** Максимум прокрутки без резинки (лэйаут ленты за жест не меняется —
   * кэш на жест, чтобы поле-расширение резинки не кормило само себя). */
  readonly baseMax: number;
  dragging: boolean;
  readonly tracker: VelocityTracker;
};

/** Дробный максимум прокрутки без резинки: layout-ширина контента по rect
 * последней карточки (scrollWidth/offsetWidth — целые и режут дробные
 * ширины fluid-плиток: 4×165.33 дают extent 685.33, целый scrollWidth —
 * 685). rect текущий — активный трансформ-вылет ЛЕВОЙ резинки
 * компенсируется (shiftPx); правая резинка поле-расширение, rect не
 * двигает. */
function baseMaxScrollLeft(rail: HTMLDivElement, shiftPx: number): number {
  const content = rail.firstElementChild as HTMLElement | null;
  const paddingLeft = Number.parseFloat(getComputedStyle(rail).paddingLeft) || 0;
  const last = content?.lastElementChild as HTMLElement | null;
  const contentWidth =
    content !== null && last !== null
      ? last.getBoundingClientRect().right - shiftPx - content.getBoundingClientRect().left
      : (content?.scrollWidth ?? rail.scrollWidth);
  return (
    Math.max(paddingLeft + contentWidth, rail.clientWidth) - rail.clientWidth
  );
}

/** Эпсилон субпиксельного клампа: точка снапа у самой границы (дробные
 * ширины fluid-плиток ставят вторую карточку ровно на максимум — 173.33 при
 * максимуме 173.33) не клампится вниз. Целый кламп ставил финальную запись
 * scrollLeft на 0.33px МИМО снап-позиции Chrome — переподключение
 * mandatory после доезда реснапило ленту к запомненной цели (первой
 * карточке): доезд доезжал, а восстановление snap уводило ленту в начало
 * (баг 18.10). */
const SNAP_CLAMP_EPSILON_PX = 1;

/** Snap-позиции детей в пространстве scrollLeft: карточки снапятся левым
 * краем на линию `scroll-padding-left` (bleed ≤560 держит 24px). Замер —
 * layout-разность offsetLeft'ов ребёнка и контента (общий offsetParent):
 * оба значения чисто лэйаут — пресс-скейл нажатой карточки (`.pressable`,
 * план 007) и трансформ-вылет резинки в геометрию не текут. Максимум —
 * дробный (baseMaxScrollLeft). Точки клампятся в достижимый диапазон с
 * субпиксельным эпсилоном: хвост ленты снапится к концу без пустот, как у
 * натива. */
function collectSnapPoints(
  rail: HTMLDivElement,
  content: HTMLDivElement,
  shiftPx: number,
): number[] {
  const style = getComputedStyle(rail);
  const scrollPadding = Number.parseFloat(style.scrollPaddingLeft) || 0;
  const railPaddingLeft = Number.parseFloat(style.paddingLeft) || 0;
  const maxScrollLeft = baseMaxScrollLeft(rail, shiftPx);
  return Array.from(content.children).map((child) => {
    const layoutOffset =
      (child as HTMLElement).offsetLeft - content.offsetLeft;
    const point = railPaddingLeft + layoutOffset - scrollPadding;
    const clamped =
      point > maxScrollLeft && point - maxScrollLeft > SNAP_CLAMP_EPSILON_PX
        ? maxScrollLeft
        : point;
    return Math.max(clamped, 0);
  });
}

export function useSnapRailDrag(): SnapRailDragControls {
  // Ручная мемоизация (стандарт React Compiler): колбэки живут в нативных
  // window-слушателях, навешиваемых на pointerdown, — пересоздание замыканий
  // между рендерами осиротило бы слушатели идущего жеста (прецедент PTR).
  const railRef = useRef<HTMLDivElement | null>(null);
  const contentRef = useRef<HTMLDivElement | null>(null);
  const gestureRef = useRef<Gesture | null>(null);
  // RAW-позиция жеста (без клампа) — с неё проецируется остановка.
  const rawRef = useRef(0);
  // Текущий визуальный вылет резинки, px (подписан: + влево от начала).
  const rubberRef = useRef(0);
  const scrollAnimRef = useRef<AnimationPlaybackControls | null>(null);
  const rubberAnimRef = useRef<AnimationPlaybackControls | null>(null);
  const cleanupRef = useRef<() => void>(() => {});
  const reducedMotion = useReducedMotion();
  const reducedMotionRef = useRef(reducedMotion);
  useEffect(() => {
    reducedMotionRef.current = reducedMotion;
  }, [reducedMotion]);

  // Off-switch snap'а на время жеста и возврат после доезда; курсор здесь
  // же — один реставратор на все inline-стили жеста. user-select — тоже
  // жест-стиль (#1436): подавление drag-select только на активном жесте.
  const restoreSnap = useCallback((element: HTMLDivElement): void => {
    element.style.scrollSnapType = "";
    element.style.scrollBehavior = "";
    element.style.cursor = "";
    element.style.userSelect = "";
  }, []);

  const clearRubber = useCallback((): void => {
    rubberRef.current = 0;
    const content = contentRef.current;
    if (content !== null) {
      content.style.transform = "";
      content.style.marginRight = "";
      const last = content.lastElementChild as HTMLElement | null;
      if (last !== null) last.style.marginRight = "";
    }
  }, []);

  /** Резинка: вылет за край, px контента. Правый край расширяет диапазон
   * прокрутки НАСТОЯЩИМ полем margin-right НА ПОСЛЕДНЕЙ КАРТОЧКЕ: поле
   * после карточек растит scrollWidth (он считается по детям) — scrollLeft
   * уходит за обычный максимум нативно, гэп после ленты и есть резинка.
   * (Поле на самом контент-диве работало, пока див был max-content; с
   * w-full див уже 100% ширины, а карточки торчат из него — margin дива
   * прячется за торчащими детьми и диапазон не расширяет: тянешь за край —
   * лента стоит/уезжает от пальца, «не липкий край» 18.10.) Трансформ тут
   * ломал бы математику — Chrome считает трансформированные границы в
   * scrollable overflow, сжимает scrollWidth и клампит scrollLeft (спираль
   * «тянешь — а лента стоит»). Левый край — трансформ: в LTR левое
   * переполнение в диапазон не входит, клампить нечему. Возврат — доезд
   * scrollLeft к снапу (правый) и спад трансформа (левый). */
  const applyRubber = useCallback((overshootPx: number): void => {
    rubberRef.current = overshootPx;
    const content = contentRef.current;
    const rail = railRef.current;
    if (content === null || rail === null) return;
    const viewport = rail.clientWidth;
    const last = content.lastElementChild as HTMLElement | null;
    if (overshootPx === 0 || viewport <= 0) {
      content.style.transform = "";
      content.style.marginRight = "";
      if (last !== null) last.style.marginRight = "";
      return;
    }
    if (overshootPx > 0) {
      content.style.transform = "";
      if (last !== null) {
        last.style.marginRight = `${rubberBandOffset(overshootPx, viewport)}px`;
      }
      // Синхронный лэйаут: запись scrollLeft ниже должна клампиться уже
      // против расширенного максимума (только пока жест за краем).
      void rail.scrollWidth;
      return;
    }
    if (last !== null) last.style.marginRight = "";
    content.style.transform = `translateX(${rubberBandOffset(-overshootPx, viewport)}px)`;
  }, []);

  const stopAnimations = useCallback((): void => {
    scrollAnimRef.current?.stop();
    scrollAnimRef.current = null;
    rubberAnimRef.current?.stop();
    rubberAnimRef.current = null;
  }, []);

  /** Доезд к snap-цели: проекция остановки по RAW-скорости (d=0.998),
   * ближайшая к ней точка, критическое демпфирование без перелёта со
   * стартом от скорости отпускания (criticalSettle). Резинка (если жест
   * отпущен за краем) возвращается в 0 параллельным докатом; snap
   * восстанавливается, когда доедут оба. */
  const settleToSnap = useCallback(
    (velocityPxPerS: number): void => {
      const rail = railRef.current;
      if (rail === null) return;
      const content = contentRef.current;
      // Текущий трансформ-вылет левой резинки — компенсация в rect-замере
      // (правая резинка поле-расширение, rect не двигает).
      const shiftPx =
        rubberRef.current < 0
          ? rubberBandOffset(-rubberRef.current, rail.clientWidth)
          : 0;
      const snapPoints =
        content !== null ? collectSnapPoints(rail, content, shiftPx) : [];
      if (content === null || snapPoints.length === 0) {
        clearRubber();
        restoreSnap(rail);
        return;
      }
      const ideal = projectStopPoint(rawRef.current, velocityPxPerS);
      const target = pickSnapTarget(ideal, snapPoints);
      if (target === null) {
        clearRubber();
        restoreSnap(rail);
        return;
      }
      if (reducedMotionRef.current) {
        // Канон §8: без инерционной проекции — короткий докат к БЛИЖАЙШЕМУ
        // снапу (150мс), резинка гасится тем же докатом.
        const nearest = pickSnapTarget(rail.scrollLeft, snapPoints);
        const from = rail.scrollLeft;
        const fromRubber = rubberRef.current;
        if (nearest !== null && Math.abs(nearest - from) > 0.5) {
          animate(from, nearest, {
            duration: REDUCED_SETTLE_S,
            ease: DL_EASE,
            onUpdate: (latest) => {
              rail.scrollLeft = latest;
            },
            onComplete: () => {
              clearRubber();
              restoreSnap(rail);
            },
          });
          return;
        }
        if (fromRubber !== 0) {
          animate(fromRubber, 0, {
            duration: REDUCED_SETTLE_S,
            ease: DL_EASE,
            onUpdate: (latest) => applyRubber(latest),
            onComplete: () => {
              clearRubber();
              restoreSnap(rail);
            },
          });
          return;
        }
        if (nearest !== null) {
          rail.scrollLeft = nearest;
        }
        clearRubber();
        restoreSnap(rail);
        return;
      }
      let done = 0;
      const bothComplete = (): void => {
        done += 1;
        if (done < 2) return;
        scrollAnimRef.current = null;
        rubberAnimRef.current = null;
        clearRubber();
        restoreSnap(rail);
      };
      const scrollSettle = criticalSettle(rail.scrollLeft, target, velocityPxPerS);
      if (scrollSettle.durationMs === 0) {
        rail.scrollLeft = target;
        bothComplete();
      } else {
        scrollAnimRef.current = animate(rail.scrollLeft, target, {
          duration: scrollSettle.durationMs / 1000,
          ease: scrollSettle.ease,
          onUpdate: (latest) => {
            rail.scrollLeft = latest;
          },
          onComplete: bothComplete,
        });
      }
      const fromRubber = rubberRef.current;
      if (fromRubber !== 0) {
        // Возврат резинки — пружина с лёгким отскоком (damping ~0.8 —
        // таблица Apple для momentum-жестов; канон §8: отскок живёт только
        // в резинке) и velocity handoff: пружина стартует со скоростью
        // отпускания в сторону покоя. Короткий возврат из-под пальца
        // критическим доездом читался резким спадом без «липкости» края.
        const towardRubber = Math.max(
          -Math.sign(fromRubber) * velocityPxPerS,
          0,
        );
        rubberAnimRef.current = animate(fromRubber, 0, {
          type: "spring",
          bounce: 0.2,
          duration: 0.4,
          velocity: -Math.sign(fromRubber) * towardRubber,
          onUpdate: (latest) => applyRubber(latest),
          onComplete: bothComplete,
        });
      } else {
        bothComplete();
      }
    },
    [applyRubber, clearRubber, restoreSnap],
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
      const rail = railRef.current;
      if (rail === null) return;
      if (event.pointerType !== "mouse" && event.pointerType !== "pen") return;
      if (event.button !== 0) return;
      if (getComputedStyle(rail).direction === "rtl") return;
      // Новый жест главнее бегущего доезда (как в PTR) — гасим; snap ещё
      // выключен, на pointerup вернём, даже если до порога дело не дошло.
      stopAnimations();
      clearRubber();

      const gesture: Gesture = {
        pointerId: event.pointerId,
        startX: event.clientX,
        startY: event.clientY,
        startScrollLeft: rail.scrollLeft,
        baseMax: baseMaxScrollLeft(rail, 0),
        dragging: false,
        tracker: createVelocityTracker(),
      };
      rawRef.current = rail.scrollLeft;
      gesture.tracker.add(event.timeStamp, rawRef.current);
      gestureRef.current = gesture;

      const onMove = (move: globalThis.PointerEvent): void => {
        const current = gestureRef.current;
        const railElement = railRef.current;
        if (
          current === null ||
          railElement === null ||
          move.pointerId !== current.pointerId
        ) {
          return;
        }
        const dx = move.clientX - current.startX;
        const dy = move.clientY - current.startY;
        if (!current.dragging) {
          if (!isHorizontalDrag(dx, dy)) return;
          current.dragging = true;
          railElement.style.scrollSnapType = "none";
          railElement.style.scrollBehavior = "auto";
          railElement.style.cursor = "grabbing";
          // Подавление drag-select реального ввода (#1436): совместимые
          // mousemove preventDefault ниже гасит, но выделение в Chromium
          // ведётся мимо них — гасит user-select:none на жесте. Очищается
          // выделение, успевшее начаться до порога (вертикальный забег
          // мыши порог не берёт и его не гасит). Жест-стиль — реставратор
          // снимет вместе с snap/cursor.
          railElement.style.userSelect = "none";
          document.getSelection()?.removeAllRanges();
          try {
            // Capture гасит hover-мигание карточек под курсором; click после
            // capture целится в ленту — но он и так гасится после drag.
            railElement.setPointerCapture(current.pointerId);
          } catch {
            // Указатель уже ушёл — жест доживёт без capture.
          }
        }
        // Совместимые mousemove гасятся — часть побочных дефолтов жеста.
        move.preventDefault();
        // RAW-позиция без клампа: скорость считается и в резинке, где
        // scrollLeft уже стоит на границе. Резинка — ПЕРЕД записью
        // scrollLeft: на правом краю она расширяет диапазон (поле), и
        // кламп записи уже против расширенного максимума.
        const raw = current.startScrollLeft - dx;
        rawRef.current = raw;
        applyRubber(raw < 0 ? raw : Math.max(raw - current.baseMax, 0));
        railElement.scrollLeft = Math.min(
          Math.max(raw, 0),
          railElement.scrollWidth - railElement.clientWidth,
        );
        current.tracker.add(move.timeStamp, raw);
      };

      const finish = (velocityPxPerS: number, ride: boolean): void => {
        const railElement = railRef.current;
        cleanupRef.current();
        gestureRef.current = null;
        if (railElement === null) return;
        // Украденный жест (pointercancel): мгновенно вернуть snap и резинку,
        // браузер беззвучно встанет в ближайшую точку.
        if (!ride) {
          stopAnimations();
          clearRubber();
          restoreSnap(railElement);
          return;
        }
        // Клик (порог не взят) поверх остановленного доезда — не рывком, а
        // коротким микродоездом к ближайшей точке (velocity 0).
        if (!gesture.dragging) {
          settleToSnap(0);
          return;
        }
        suppressNextClick(railElement);
        settleToSnap(velocityPxPerS);
      };

      const onUp = (up: globalThis.PointerEvent): void => {
        const current = gestureRef.current;
        if (current === null || up.pointerId !== current.pointerId) return;
        // Скорость на МОМЕНТ отпускания: сэмплы старше окна от него мертвы —
        // пауза перед отпусканием оседает в ближайшем снапе, а не флитит.
        finish(current.tracker.velocityPxPerS(up.timeStamp), true);
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
    [applyRubber, clearRubber, restoreSnap, settleToSnap, stopAnimations, suppressNextClick],
  );

  const onDragStart = useCallback((event: DragEvent<HTMLDivElement>): void => {
    event.preventDefault();
  }, []);

  // Колесо/тачпад поверх бегущего доезда: гасим анимации, гасим резинку и
  // возвращаем snap — нативный скролл колеса сам встанет в ближайшую точку.
  useEffect(() => {
    const rail = railRef.current;
    if (rail === null) return;
    const onWheel = (): void => {
      if (scrollAnimRef.current === null && rubberAnimRef.current === null) {
        return;
      }
      stopAnimations();
      clearRubber();
      restoreSnap(rail);
    };
    rail.addEventListener("wheel", onWheel, { passive: true });
    return () => {
      rail.removeEventListener("wheel", onWheel);
      cleanupRef.current();
      stopAnimations();
      clearRubber();
    };
  }, [clearRubber, restoreSnap, stopAnimations]);

  return { railRef, contentRef, onPointerDown, onDragStart };
}
