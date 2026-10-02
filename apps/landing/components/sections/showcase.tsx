"use client";

import Image, { type StaticImageData } from "next/image";
import { useCallback, useEffect, useRef, useState, type CSSProperties } from "react";
import { LandingLink } from "@/components/button";
import { Reveal } from "@/components/reveal";
import house from "@/assets/sections/carousel-house.webp";
import warehouse from "@/assets/sections/carousel-warehouse.webp";
import flat from "@/assets/sections/carousel-flat.webp";
import room from "@/assets/sections/carousel-room.webp";
import shop from "@/assets/sections/carousel-shop.webp";
import avatarHouse from "@/assets/sections/carousel-avatar-house.webp";
import arrowRight from "@/assets/icons/icon-arrow-right-small.svg";
import s from "./showcase.module.css";

// «Создайте карточку своей недвижимости» — макеты 2967-75653 (ПК),
// 3005-78445 (планшет), 3008-79114 (мобила) + аннотация владельца:
// бесконечная прокрутка, Shift+Scroll, drag мышью, клик по боковой
// картинке или плашке приводит её к центру, плашка к центру
// увеличивается, боковые уменьшены до 80% (мобила = планшет × 0.6).
//
// Механика движения — по замерам лент Яндекса (Swiper speed 360 на
// pay.yandex.ru/business, октябрь 2026): единственная непрерывная позиция
// p (та же терминология, что был scrollLeft у нативного скроллера),
// все программные переходы — 360мс ease-out cubic-bezier(0,0,0.58,1),
// посадка точно в позицию покоя — без инерции, перелётов и «магнитных»
// доводок. Нативного скролла нет вовсе: фотолента — overflow:hidden,
// p кладётся в translate3d дорожки; плашки — синхронный ряд, который
// центруется по той же p. Drag (мышь и тач) — 1:1 за пальцем, отпуск —
// докат 360мс к ближайшей карточке; быстрый бросок добавляет одну
// карточку по направлению (порог по скорости, как у Swiper). Клик по
// боковой карточке/плашке ведёт её в центр одним переходом независимо
// от дистанции; клик и wheel во время анимации игнорируются, drag
// перехватывает движение. Shift+Scroll и горизонтальный трекпад — шаг
// ровно на одну карточку, вертикальное колесо скроллит страницу.
// Бесконечность — 3 копии набора с тихой пересадкой в среднюю на покое.
// Морфинг размеров — CSS --k (0 бок / 1 центр); высоты обоих рядов
// зафиксированы (var(--h) / var(--plate-h)), поэтому контент под
// каруселью не двигается ни при каком движении.

type Spec = readonly [label: string, value: string];

type Card = {
  key: string;
  name: string;
  address: string;
  photo: StaticImageData;
  avatar: StaticImageData;
  specs: readonly Spec[];
};

const CARDS: readonly Card[] = [
  {
    key: "house",
    name: "Загородный дом",
    address: "Краснодар, улица Гагарина",
    photo: house,
    avatar: avatarHouse,
    specs: [
      ["Площадь участка", "3 сотки"],
      ["Тип участка", "Садовый"],
      ["Этажность дома", "3 этажа"],
      ["Материал", "Кирпичный"],
      ["Санузел", "В доме"],
      ["Год постройки дома", "2010"],
    ],
  },
  {
    key: "warehouse",
    name: "Склад 150 м²",
    address: "Коломна, улица Пионерская",
    photo: warehouse,
    avatar: warehouse,
    specs: [
      ["Общая площадь", "150 м²"],
      ["Вход", "Отдельный"],
      ["Тип здания", "Отдельное здание"],
    ],
  },
  {
    key: "flat",
    name: "2-комнатная на Ленина",
    address: "Москва, улица Ленина",
    photo: flat,
    avatar: flat,
    specs: [
      ["Общая площадь", "45,5 м²"],
      ["Жилая площадь", "30,2 м²"],
      ["Площадь кухни", "10 м²"],
      ["Этаж", "6 из 12"],
      ["Санузел", "Совмещенный"],
      ["Балкон", "Есть"],
      ["Ремонт", "Евро"],
      ["Высота потолков", "2,2 м"],
      ["Год постройки дома", "2002"],
    ],
  },
  {
    key: "room",
    name: "Комната",
    address: "Екатеринбург, улица Гоголя",
    photo: room,
    avatar: room,
    specs: [
      ["Площадь комнаты", "20 м²"],
      ["Этаж", "7 из 10"],
      ["Санузел", "Раздельный"],
      ["Балкон", "Нет"],
      ["Ремонт", "Косметический"],
      ["Высота потолков", "2,3 м"],
      ["Год постройки дома", "2004"],
    ],
  },
  {
    key: "shop",
    name: "Помещение под магазин",
    address: "Санкт-Петербург, Кирпичный переулок",
    photo: shop,
    avatar: shop,
    specs: [
      ["Общая площадь", "89,5 м²"],
      ["Тип здания", "Торговый центр"],
      ["Этаж", "2 из 4"],
      ["Вход", "Общий"],
      ["Ремонт", "Косметический"],
    ],
  },
];

const N = CARDS.length;
const COPIES = 3;
const G = N * COPIES; // узлов в ленте
const INITIAL = N + 2; // «2-комнатная на Ленина»: средняя копия (слайды N..2N−1), карта 2
const STEP_MS = 360; // длительность любого перехода — как у Яндекса (замер)

// Ширины состояний [боковая, центральная] для JS-математики — зеркало
// CSS-переменных showcase.module.css; D — дистанция морфинга k, равна
// шагу до соседней карточки (cardSide+gap) на всех ярусах: морфинг
// завершается точно в момент прибытия (как у Яндекса), на покое k
// строго 0/1 — бок ровно 80%, центр ровно по центру вьюпорта.
const METRICS = {
  desk: { cardSide: 800, card: 1000, gap: 20, plateSide: 360, plate: 450, plateGap: 16, D: 820 },
  tab: { cardSide: 500, card: 500, gap: 12, plateSide: 345, plate: 400, plateGap: 12, D: 512 },
  mob: { cardSide: 300, card: 300, gap: 12, plateSide: 207, plate: 240, plateGap: 8, D: 312 },
} as const;
type Tier = keyof typeof METRICS;

/* Точная модель позиций покоя. В состоянии покоя активная карточка —
   центральная ширина, остальные — боковые, поэтому позиция p,
   центрирующая слайд g (копия c = floor(g/N), карта i = g mod N):
   S(g) = c×copyPlain + i×(sideW+gap) + centerW/2 − viewportW/2,
   copyPlain = N×(sideW+gap). Сдвиг на copyPlain переводит S(g)↔S(g±N)
   точно (соседние копии визуально идентичны) — на нём держится
   бесконечность. Все целевые позиции считаются из модели, не из живого
   лэйаута: ширины карточек меняются при морфинге, живые смещения в
   момент старта анимации врали бы на (centerW−sideW)/2. */
function snapModel(tier: Tier, viewportW: number) {
  const m = METRICS[tier];
  const step = m.cardSide + m.gap;
  const copyPlain = N * step;
  const snap = (g: number) =>
    Math.floor(g / N) * copyPlain + (g % N) * step + m.card / 2 - viewportW / 2;
  const snapArr = Array.from({ length: G }, (_, g) => snap(g));
  return { m, step, copyPlain, snapArr };
}

const clamp = (v: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, v));

function tierOf(): Tier {
  if (window.matchMedia("(min-width: 1200px)").matches) return "desk";
  if (window.matchMedia("(min-width: 481px)").matches) return "tab";
  return "mob";
}

// Узел ленты, ближайший к позиции p, — по позициям покоя модели.
function nearestIndex(p: number, snapArr: readonly number[]): number {
  let best = 0;
  let bestDist = Math.abs((snapArr[0] ?? 0) - p);
  for (let g = 1; g < snapArr.length; g += 1) {
    const d = Math.abs((snapArr[g] ?? 0) - p);
    if (d < bestDist) {
      bestDist = d;
      best = g;
    }
  }
  return best;
}

// CSS ease-out cubic-bezier(0, 0, 0.58, 1) — кривая переходов Яндекса
// (совпадение с замером в пределах ~2%): решаем параметр кривой Ньютоном.
function easeOut(t: number): number {
  if (t <= 0) return 0;
  if (t >= 1) return 1;
  const x2 = 0.58;
  const px = (s: number) => 3 * s * s * (1 - s) * x2 + s * s * s;
  const dx = (s: number) => 6 * s * (1 - s) * x2 + 3 * s * s;
  const py = (s: number) => 3 * s * s * (1 - s) + s * s * s;
  let curve = t;
  for (let i = 0; i < 8; i += 1) {
    const err = px(curve) - t;
    const slope = dx(curve);
    if (Math.abs(err) < 1e-6 || Math.abs(slope) < 1e-6) break;
    curve -= err / slope;
  }
  return py(clamp(curve, 0, 1));
}

// Мутабельное состояние ленты — создается в mount-эффекте и меняется
// только в обработчиках (канон refs лендинга: никакого доступа в рендере)
type EngineState = {
  p: number;
  k: number[];
  anim: { from: number; to: number; t0: number; dur: number } | null;
  raf: number;
  dragging: boolean;
  dragMoved: boolean;
  pointerId: number | null;
  dragStartX: number;
  dragStartP: number;
  dragSamples: Array<{ t: number; x: number }>;
  lastWheelT: number;
};

// Разрыв между горизонтальными wheel-событиями, после которого поток
// считается новым жестом. Моментум трекпада живёт заметно дольше 360мс
// анимации (замер: 24 события за ~850мс проматывали 4 карточки) —
// глотания на время анимации недостаточно, хвост того же жеста
// отсекается окном тишины (решение владельца 02.10).
const WHEEL_GESTURE_GAP_MS = 300;

export function Showcase() {
  const [mounted, setMounted] = useState(false);
  const zoneRef = useRef<HTMLDivElement>(null); // колонка карусели (плашки + фото)
  const viewportRef = useRef<HTMLDivElement>(null); // окно фотоленты (full-bleed 100vw)
  const photoTrackRef = useRef<HTMLDivElement>(null); // дорожка фото, translate3d(−p)
  const stripRef = useRef<HTMLDivElement>(null); // дорожка плашек, центруется по p
  const slideNodes = useRef<Array<HTMLElement | null>>([]);
  const plateNodes = useRef<Array<HTMLElement | null>>([]);
  const stateRef = useRef<EngineState | null>(null);
  const reducedRef = useRef(false);

  const reduced = useCallback(() => reducedRef.current, []);

  // Один проход отрисовки: k по дистанции до позиций покоя, транслейт
  // фотодорожки, центровка ряда плашек. Источник истины — st.p.
  const render = useCallback(() => {
    const viewport = viewportRef.current;
    const photoTrack = photoTrackRef.current;
    const strip = stripRef.current;
    const zone = zoneRef.current;
    const st = stateRef.current;
    if (!viewport || !photoTrack || !strip || !zone || !st) return;
    const { m, snapArr } = snapModel(tierOf(), viewport.clientWidth);
    const p = st.p;
    const k = st.k;
    let dirty = false;
    for (let g = 0; g < G; g += 1) {
      const nk = clamp(1 - Math.abs(p - (snapArr[g] ?? 0)) / m.D, 0, 1);
      if (Math.abs(nk - (k[g] ?? 0)) > 0.002) {
        k[g] = nk;
        dirty = true;
      }
    }
    if (dirty) {
      for (let g = 0; g < G; g += 1) {
        const kk = (k[g] ?? 0).toFixed(3);
        slideNodes.current[g]?.setAttribute("style", `--k:${kk}`);
        plateNodes.current[g]?.setAttribute("style", `--k:${kk}`);
      }
    }
    photoTrack.style.transform = `translate3d(${(-p).toFixed(2)}px,0,0)`;
    // Плашки: транслейт линейно по p между позициями покоя — плашечный
    // ряд скользит с той же кривой, что и фото (frac линеен по p), и
    // прибывает ровно в центр. Формула через накопление текущих ширин
    // здесь не годится: при смене активного индекса она даёт скачок
    // (centerW−sideW)/2 — найдено замером (44.7px на прибытии).
    const pitch = m.plateSide + m.plateGap;
    let gLo = 0;
    while (gLo < G - 1 && (snapArr[gLo + 1] ?? 0) <= p) gLo += 1;
    const next = snapArr[Math.min(gLo + 1, G - 1)] ?? 0;
    const span = Math.max(next - (snapArr[gLo] ?? 0), 1);
    const frac = clamp((p - (snapArr[gLo] ?? 0)) / span, 0, 1);
    const tx = zone.clientWidth / 2 - (gLo * pitch + m.plate / 2) - frac * pitch;
    strip.style.transform = `translateX(${tx.toFixed(1)}px)`;
  }, []);

  // Тихая пересадка в среднюю копию — только на покое (позиции покоя
  // соседних копий совпадают точно, сдвиг невидим).
  const renormalize = useCallback(() => {
    const viewport = viewportRef.current;
    const st = stateRef.current;
    if (!viewport || !st) return;
    const { snapArr, copyPlain } = snapModel(tierOf(), viewport.clientWidth);
    let best = nearestIndex(st.p, snapArr);
    let shifted = false;
    while (best < 2) {
      best += N;
      st.p += copyPlain;
      shifted = true;
    }
    while (best > G - 3) {
      best -= N;
      st.p -= copyPlain;
      shifted = true;
    }
    if (shifted) render();
  }, [render]);

  const frameLoop = useCallback(() => {
    const st = stateRef.current;
    if (!st) return;
    const step = function stepFn(t: number) {
      const a = st.anim;
      if (!a) return;
      const x = a.dur <= 0 ? 1 : clamp((t - a.t0) / a.dur, 0, 1);
      st.p = a.from + (a.to - a.from) * easeOut(x);
      render();
      if (x < 1) {
        st.raf = requestAnimationFrame(stepFn);
      } else {
        st.anim = null;
        renormalize();
      }
    };
    st.raf = requestAnimationFrame(step);
  }, [render, renormalize]);

  // Единственная анимация движения: фиксированные 360мс ease-out до
  // точной позиции покоя, независимо от дистанции.
  const animateTo = useCallback(
    (to: number) => {
      const st = stateRef.current;
      if (!st) return;
      if (Math.abs(st.p - to) < 0.5) {
        st.anim = null;
        st.p = to;
        render();
        renormalize();
        return;
      }
      st.anim = { from: st.p, to, t0: performance.now(), dur: reduced() ? 0 : STEP_MS };
      cancelAnimationFrame(st.raf);
      frameLoop();
    },
    [frameLoop, reduced, render, renormalize],
  );

  // Выбор карточки по индексу узла ленты: кликнутая едет в центр одним
  // переходом, сколько бы шагов ни было (кратчайший обход кольца копий).
  // Клик во время анимации или drag игнорируется — как у Яндекса.
  const selectNode = useCallback(
    (g: number) => {
      const viewport = viewportRef.current;
      const st = stateRef.current;
      if (!viewport || !st || st.anim || st.dragging) return;
      const { snapArr } = snapModel(tierOf(), viewport.clientWidth);
      const cur = nearestIndex(st.p, snapArr);
      let delta = g - cur;
      if (delta > G / 2) delta -= G;
      if (delta < -G / 2) delta += G;
      const target = clamp(cur + delta, 0, G - 1);
      animateTo(snapArr[target] ?? st.p);
    },
    [animateTo],
  );

  useEffect(() => {
    const rm = window.matchMedia("(prefers-reduced-motion: reduce)");
    reducedRef.current = rm.matches;
    const onRmChange = () => {
      reducedRef.current = rm.matches;
    };
    rm.addEventListener("change", onRmChange);
    return () => rm.removeEventListener("change", onRmChange);
  }, []);

  // Стартовая центровка и пересборка базы на смену яруса.
  useEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport) return;
    stateRef.current ??= {
      p: 0,
      k: new Array<number>(G).fill(0),
      anim: null,
      raf: 0,
      dragging: false,
      dragMoved: false,
      pointerId: null,
      dragStartX: 0,
      dragStartP: 0,
      dragSamples: [],
      lastWheelT: 0,
    };
    const measure = () => {
      const st0 = stateRef.current;
      if (!st0) return;
      st0.anim = null;
      st0.p = snapModel(tierOf(), viewport.clientWidth).snapArr[INITIAL] ?? 0;
      st0.k.fill(0);
      render();
    };
    measure();
    setMounted(true);
    // Ресайз окна и смены яруса: позиция покоя пересчитывается под новую
    // геометрию (ближайшая карточка встаёт ровно в центр). Слушатель один
    // — matchMedia-сравнения тут не работают: к моменту события change
    // MediaQueryList.matches уже возвращает новое значение.
    let resizeRaf = 0;
    const onViewportChange = () => {
      cancelAnimationFrame(resizeRaf);
      resizeRaf = requestAnimationFrame(() => {
        const st0 = stateRef.current;
        if (!st0 || st0.dragging) return;
        const { snapArr, copyPlain } = snapModel(tierOf(), viewport.clientWidth);
        let best = nearestIndex(st0.p, snapArr);
        let p = snapArr[best] ?? st0.p;
        while (best < 2) {
          best += N;
          p += copyPlain;
        }
        while (best > G - 3) {
          best -= N;
          p -= copyPlain;
        }
        st0.anim = null;
        cancelAnimationFrame(st0.raf);
        st0.p = p;
        st0.k.fill(0);
        render();
      });
    };
    window.addEventListener("resize", onViewportChange);
    return () => {
      window.removeEventListener("resize", onViewportChange);
      cancelAnimationFrame(resizeRaf);
      const st = stateRef.current;
      if (st) {
        cancelAnimationFrame(st.raf);
        st.anim = null;
      }
    };
  }, [render]);

  // Drag мышью и тачем — 1:1 за пальцем, без инерции; отпуск — докат
  // 360мс к ближайшей карточке, быстрый бросок добавляет одну по
  // направлению. Указатель каптурится окном ленты, клик разрешается
  // хит-тестом (цель после капчера — окно, не карточка).
  useEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport) return;
    const st = () => stateRef.current;

    const onPointerDown = (e: PointerEvent) => {
      if (e.pointerType === "mouse" && e.button !== 0) return;
      const s0 = st();
      if (!s0) return;
      s0.dragging = true;
      s0.dragMoved = false;
      s0.pointerId = e.pointerId;
      s0.dragStartX = e.clientX;
      s0.dragStartP = s0.p;
      s0.dragSamples = [{ t: performance.now(), x: e.clientX }];
      s0.anim = null; // drag перехватывает движение
      cancelAnimationFrame(s0.raf);
      viewport.classList.add(s.dragging ?? "");
      viewport.setPointerCapture(e.pointerId);
    };

    const onPointerMove = (e: PointerEvent) => {
      const s1 = st();
      if (!s1 || !s1.dragging || e.pointerId !== s1.pointerId) return;
      const dx = e.clientX - s1.dragStartX;
      if (Math.abs(dx) > 5) s1.dragMoved = true;
      const { snapArr } = snapModel(tierOf(), viewport.clientWidth);
      s1.p = clamp(s1.dragStartP - dx, snapArr[1] ?? 0, snapArr[G - 2] ?? 0);
      render();
      s1.dragSamples.push({ t: performance.now(), x: e.clientX });
      if (s1.dragSamples.length > 6) s1.dragSamples.shift();
    };

    const release = (e: PointerEvent, flick: boolean) => {
      const s2 = st();
      if (!s2 || !s2.dragging || e.pointerId !== s2.pointerId) return;
      s2.dragging = false;
      s2.pointerId = null;
      viewport.classList.remove(s.dragging ?? "");
      if (!s2.dragMoved) {
        // клик: цель — кликнутая карточка (data-slide-idx = индекс узла)
        const el = document.elementFromPoint(e.clientX, e.clientY);
        const host = el?.closest<HTMLElement>("[data-slide-idx]");
        if (host) selectNode(Number(host.dataset.slideIdx));
        return;
      }
      const { snapArr } = snapModel(tierOf(), viewport.clientWidth);
      let best = nearestIndex(s2.p, snapArr);
      if (flick) {
        // скорость по окну ~100мс: последний сэмпл к моменту up почти
        // всегда стоит на месте (движения склеиваются в кадр), оконная
        // скорость ловит бросок даже при остановившемся указателе
        const now = performance.now();
        const windowStart = now - 100;
        const last = s2.dragSamples.at(-1);
        const ref = s2.dragSamples.find((smp) => smp.t >= windowStart);
        if (last && ref && last.t - ref.t >= 0 && last.t >= windowStart) {
          const dt = Math.max(now - ref.t, 1);
          const vP = -(e.clientX - ref.x) / dt; // скорость p, px/мс
          // бросок двигает ленту на одну карточку, только если тягой
          // порог половины шага не взят (иначе ближайшая уже следующая)
          if (Math.abs(vP) > 0.5 && best === nearestIndex(s2.dragStartP, snapArr)) {
            best = clamp(best + Math.sign(vP), 1, G - 2);
          }
        }
      }
      animateTo(snapArr[best] ?? s2.p);
    };

    const onPointerUp = (e: PointerEvent) => release(e, true);
    const onPointerCancel = (e: PointerEvent) => release(e, false);

    viewport.addEventListener("pointerdown", onPointerDown);
    viewport.addEventListener("pointermove", onPointerMove);
    viewport.addEventListener("pointerup", onPointerUp);
    viewport.addEventListener("pointercancel", onPointerCancel);
    return () => {
      viewport.removeEventListener("pointerdown", onPointerDown);
      viewport.removeEventListener("pointermove", onPointerMove);
      viewport.removeEventListener("pointerup", onPointerUp);
      viewport.removeEventListener("pointercancel", onPointerCancel);
    };
  }, [animateTo, render, selectNode]);

  // Shift+Scroll и горизонтальный трекпад — шаг ровно на одну карточку
  // РАЗ В ЖЕСТ: первое событие после паузы длиннее окна тишины шагает,
  // весь хвост моментума (события короче окна) игнорируется. Вертикальное
  // колесо не наша ось — скроллит страницу.
  useEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport) return;
    const onWheel = (e: WheelEvent) => {
      if (Math.abs(e.deltaX) <= Math.abs(e.deltaY)) return;
      e.preventDefault();
      const st = stateRef.current;
      if (!st) return;
      const now = performance.now();
      const newGesture = now - st.lastWheelT > WHEEL_GESTURE_GAP_MS;
      st.lastWheelT = now;
      if (!newGesture || st.anim || st.dragging) return;
      const { snapArr } = snapModel(tierOf(), viewport.clientWidth);
      const cur = nearestIndex(st.p, snapArr);
      const g = clamp(cur + (e.deltaX > 0 ? 1 : -1), 1, G - 2);
      animateTo(snapArr[g] ?? st.p);
    };
    viewport.addEventListener("wheel", onWheel, { passive: false });
    return () => viewport.removeEventListener("wheel", onWheel);
  }, [animateTo]);

  return (
    <section id="showcase" className="mt-24 scroll-mt-[88px] desk:mt-[156px] desk:scroll-mt-[104px]">
      <div className="mx-auto flex max-w-[1048px] flex-col items-center gap-8 px-10 desk:max-w-[1000px] desk:gap-14 desk:px-0">
        <Reveal className="w-full">
          <div className="flex w-full flex-col items-center gap-3 text-center desk:gap-6">
            <h2 className="text-balance text-[28px] font-semibold leading-8 desk:text-h2 desk:leading-[60px]">
              Создайте карточку своей недвижимости
            </h2>
            <p className="text-balance text-s leading-5 text-gray-2 desk:text-[28px] desk:leading-8 desk:font-medium">
              Добавьте квартиру, дом, помещение
            </p>
          </div>
        </Reveal>
        <Reveal delay={100} className="w-full">
          <div
            ref={zoneRef}
            className={`${s.carousel} ${s.zone}`}
            style={{ visibility: mounted ? "visible" : "hidden" }}
          >
            <div className={s.strip}>
              <div ref={stripRef} className={s.stripTrack}>
                {Array.from({ length: COPIES }, (_, c) =>
                  CARDS.map((card, i) => {
                    const g = c * N + i;
                    return (
                      <button
                        key={`${c}-${card.key}`}
                        ref={(el) => {
                          plateNodes.current[g] = el;
                        }}
                        type="button"
                        aria-label={`Показать: ${card.name}`}
                        className={s.plate}
                        style={{ "--k": g === INITIAL ? 1 : 0 } as CSSProperties}
                        onClick={() => selectNode(g)}
                      >
                        <Image src={card.avatar} alt="" className={s.plateIcon} sizes="52px" />
                        <span className={s.plateText}>
                          <span className={s.plateName}>{card.name}</span>
                          <span className={s.plateAddress}>{card.address}</span>
                        </span>
                        <Image src={arrowRight} alt="" className={s.plateArrow} sizes="24px" />
                      </button>
                    );
                  }),
                )}
              </div>
              <div className={s.fadeStripLeft} />
              <div className={s.fadeStripRight} />
            </div>
            <div className={s.viewport}>
              <div ref={viewportRef} className={s.scroller}>
                <div ref={photoTrackRef} className={s.track}>
                  {Array.from({ length: COPIES }, (_, c) =>
                    CARDS.map((card, i) => {
                      const g = c * N + i;
                      const near = c === 1;
                      return (
                        <div
                          key={`${c}-${card.key}`}
                          ref={(el) => {
                            slideNodes.current[g] = el;
                          }}
                          data-slide-idx={g}
                          role="button"
                          tabIndex={near ? 0 : -1}
                          aria-label={`Показать: ${card.name}`}
                          className={s.slide}
                          style={{ "--k": g === INITIAL ? 1 : 0 } as CSSProperties}
                          onKeyDown={(e) => {
                            if (e.key === "Enter" || e.key === " ") {
                              e.preventDefault();
                              selectNode(g);
                            }
                          }}
                        >
                          <Image
                            src={card.photo}
                            alt=""
                            fill
                            draggable={false}
                            loading={near ? "eager" : "lazy"}
                            sizes="(min-width: 1200px) 1000px, (min-width: 481px) 500px, 300px"
                            className={s.slidePhoto}
                          />
                          <div className={s.card}>
                            <div className={s.cardGlass} />
                            <div className={s.cardName}>{card.name}</div>
                            <div className={s.cardRows}>
                              {card.specs.map(([label, value]) => (
                                <div key={label} className={s.cardRow}>
                                  <span className={s.cardLabel}>{label}</span>
                                  <span className={s.cardValue}>{value}</span>
                                </div>
                              ))}
                            </div>
                            <div className={s.cardRing} />
                          </div>
                        </div>
                      );
                    }),
                  )}
                </div>
              </div>
              <div className={s.fadePhotoLeft} />
              <div className={s.fadePhotoRight} />
            </div>
          </div>
        </Reveal>
        <Reveal delay={200}>
          <LandingLink href="/login">Попробовать</LandingLink>
        </Reveal>
      </div>
    </section>
  );
}
