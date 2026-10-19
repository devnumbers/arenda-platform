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
  type VelocityTracker,
} from "./pull-math";
import {
  criticalSettle,
  isHorizontalDrag,
  projectStopPoint,
} from "./snap-rail-physics";

/** Зеркало кривой дома --dl-ease (tokens.css) для reduced-motion-доката:
 * JS-аниматор не читает var(), дубликат вынужденный (прецедент LiveValue). */
const DL_EASE: [number, number, number, number] = [0.32, 0.72, 0, 1];
/** Reduced-motion: жест — управление (не гасится), гашение резинки
 * укорачивается до 150мс (канон §8 «укорачиваем, не отключаем»). Единица —
 * секунды (format `animate` motion). */
const REDUCED_SETTLE_S = 0.15;

/**
 * Drag-прокси мыши/пера поверх нативного горизонтального свободного
 * скролла (#1349; физика по эталону Apple; канон — DESIGN.md §8; снапа
 * нет — решение владельца 19.10 «магнитность убрать, пусть просто
 * скроллится», supersedes proximity 18.10 и mandatory 14.10).
 *
 * Контракт: `scrollLeft` ленты следует за указателем 1:1; скорость
 * считается по RAW-позиции (без клампа краёв — скорость живёт и в вязнущей
 * резинке); на отпускании точка остановки проецируется заранее
 * (projectStopPoint, d=0.998 — веб-аналог `scrollViewWillEndDragging`) —
 * БЕЗ привязки к карточным линиям — и доезжается критическим демпфированием
 * без перелёта со стартом от скорости отпускания (criticalSettle: velocity
 * handoff, адаптивная ω ≥ |v|/|d| — доезд стартует с пальца без
 * «кирпичной стены», apple-design §5).
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
 * заходит вовсе (гвард `pointerType`), momentum и резинка браузера целы.
 *
 * Сохранённые механики 94ba68fec (проверены живой приёмкой 12/12): клик
 * живёт до порога, после реального drag первый click гасится
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

/** Хвостовой отступ последней плитки (канон SnapRail, телефонный ярус
 * ≤560): after-псевдоэлемент контент-дива растит scrollWidth браузера, но
 * в rect последней карточки не входит — добавляется явно (псевдо 16px +
 * column-gap 8 = 24px, px-6). Без надбавки кламп прокси съедал бы хвост:
 * мышь доезжала бы на 24px меньше нативного тача, и резинка заводилась
 * раньше настоящего конца. Скрытое псевдо (выше 560) — display none, ноль. */
function tailPadOf(content: HTMLElement | null): number {
  if (content === null) return 0;
  const after = getComputedStyle(content, "::after");
  if (after.display === "none") return 0;
  const gap = Number.parseFloat(getComputedStyle(content).columnGap) || 0;
  return (Number.parseFloat(after.width) || 0) + gap;
}

/** Дробный максимум прокрутки без резинки: layout-ширина контента по rect
 * последней карточки (scrollWidth/offsetWidth — целые и режут дробные
 * ширины fluid-плиток: 4×165.33 дают extent 685.33, целый scrollWidth —
 * 685). rect текущий — активный трансформ-вылет ЛЕВОЙ резинки
 * компенсируется (shiftPx); правая резинка поле-расширение, rect не
 * двигает. Хвостовой отступ после карточек — tailPadOf. */
function baseMaxScrollLeft(rail: HTMLDivElement, shiftPx: number): number {
  const content = rail.firstElementChild as HTMLElement | null;
  const paddingLeft = Number.parseFloat(getComputedStyle(rail).paddingLeft) || 0;
  const last = content?.lastElementChild as HTMLElement | null;
  const contentWidth =
    content !== null && last !== null
      ? last.getBoundingClientRect().right - shiftPx - content.getBoundingClientRect().left
      : (content?.scrollWidth ?? rail.scrollWidth);
  return (
    Math.max(paddingLeft + contentWidth + tailPadOf(content), rail.clientWidth) -
    rail.clientWidth
  );
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

  // Реставратор inline-стилей жеста (курсор, scroll-behavior для
  // программных записей, user-select #1436). Снапа нет — переключать
  // нечего, свободный скролл браузера не борется с записями scrollLeft.
  const restoreGestureStyles = useCallback((element: HTMLDivElement): void => {
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

  /** Доезд по проекции остановки: цель = проецированная точка, зажатая в
   * достижимый диапазон, БЕЗ привязки к карточным линиям (снапа нет —
   * решение владельца 19.10); критическое демпфирование без перелёта со
   * стартом от скорости отпускания (criticalSettle). Резинка (если жест
   * отпущен за краем) возвращается параллельной пружиной с лёгким
   * отскоком; стили жеста снимаются, когда доедут оба. */
  const settleToProjection = useCallback(
    (velocityPxPerS: number): void => {
      const rail = railRef.current;
      if (rail === null) return;
      // Текущий трансформ-вылет левой резинки — компенсация в rect-замере
      // (правая резинка поле-расширение, rect не двигает).
      const shiftPx =
        rubberRef.current < 0
          ? rubberBandOffset(-rubberRef.current, rail.clientWidth)
          : 0;
      const maxScrollLeft = baseMaxScrollLeft(rail, shiftPx);
      const ideal = projectStopPoint(rawRef.current, velocityPxPerS);
      const target = Math.min(Math.max(ideal, 0), maxScrollLeft);
      const finishSettle = (): void => {
        stopAnimations();
        clearRubber();
        restoreGestureStyles(rail);
      };
      if (reducedMotionRef.current) {
        // Канон §8 «укорачиваем, не отключаем»: без инерционной проекции —
        // лента остаётся у пальца (записи не было — позиция уже на месте),
        // резинка гасится коротким спадом.
        rail.scrollLeft = Math.min(
          Math.max(rail.scrollLeft, 0),
          maxScrollLeft,
        );
        const fromRubber = rubberRef.current;
        if (fromRubber !== 0) {
          animate(fromRubber, 0, {
            duration: REDUCED_SETTLE_S,
            ease: DL_EASE,
            onUpdate: (latest) => applyRubber(latest),
            onComplete: finishSettle,
          });
          return;
        }
        finishSettle();
        return;
      }
      let done = 0;
      const bothComplete = (): void => {
        done += 1;
        if (done < 2) return;
        finishSettle();
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
        // в резинке) и velocity handoff при броске к покою: пружина
        // стартует со скоростью отпускания в сторону покоя (отпускание в
        // натяг — старт от нуля, тяга наружу не борется с возвратом).
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
    [applyRubber, clearRubber, restoreGestureStyles, stopAnimations],
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
          restoreGestureStyles(railElement);
          return;
        }
        // Клик (порог не взят) поверх остановленного доезда — лента
        // остаётся у пальца, резинка гасится доездом к покою (velocity 0).
        if (!gesture.dragging) {
          settleToProjection(0);
          return;
        }
        suppressNextClick(railElement);
        settleToProjection(velocityPxPerS);
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
    [applyRubber, clearRubber, restoreGestureStyles, settleToProjection, stopAnimations, suppressNextClick],
  );

  const onDragStart = useCallback((event: DragEvent<HTMLDivElement>): void => {
    event.preventDefault();
  }, []);

  // Колесо/тачпад поверх бегущего доезда: гасим анимации и резинку —
  // нативный свободный скролл колеса продолжает жест.
  useEffect(() => {
    const rail = railRef.current;
    if (rail === null) return;
    const onWheel = (): void => {
      if (scrollAnimRef.current === null && rubberAnimRef.current === null) {
        return;
      }
      stopAnimations();
      clearRubber();
      restoreGestureStyles(rail);
    };
    rail.addEventListener("wheel", onWheel, { passive: true });
    return () => {
      rail.removeEventListener("wheel", onWheel);
      cleanupRef.current();
      stopAnimations();
      clearRubber();
    };
  }, [clearRubber, restoreGestureStyles, stopAnimations]);

  return { railRef, contentRef, onPointerDown, onDragStart };
}
