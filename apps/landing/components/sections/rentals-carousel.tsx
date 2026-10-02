"use client";

import Image, { type StaticImageData } from "next/image";
import { useCallback, useEffect, useRef } from "react";
import { easeOut } from "@/components/carousel-ease";
import rentalCalendar from "@/assets/sections/rental-calendar.webp";
import rentalContacts from "@/assets/sections/rental-contacts.webp";
import rentalContract from "@/assets/sections/rental-contract.webp";
import rentalHistory from "@/assets/sections/rental-history.webp";
import rentalOverdue from "@/assets/sections/rental-overdue.webp";
import rentalReport from "@/assets/sections/rental-report.webp";
import s from "./rentals-carousel.module.css";

// Карусель «Управляйте арендой» — макеты 2967-75818 (ПК), 3005-78042
// (планшет), 3008-79124 (мобила): 6 карточек 380×550 (r40) на десктопе
// и 320×500 (r32) на планшете-мобиле; иллюстрации прижаты к низу по
// макету каждой карточки. Лента стартует от левого края контентной
// колонки (на ПК 1000px, на планшете-мобиле поле 24px) и течёт за её
// правый край до обреза вьюпортом. Точек и «полки» нет — карточки
// равные, без масштаба и приглушения.
//
// Движок — 1:1 с каруселью pay.yandex.ru/business/acquiring (замер
// 02.10): единственная позиция p в translate3d дорожки, нативного
// скролла нет; программные переходы — фиксированные 360мс ease-out
// (на ширинах <720 — 460мс, мобильный брейкпоинт той секции)
// независимо от дистанции, посадка точно в позицию покоя. Drag (мышь
// и тач) — 1:1 за указателем без инерции; на отпускании снап на
// ближайшую позицию, быстрый бросок добавляет одну карточку по
// направлению (если порог половины шага тягой не взят); за краями —
// резиновая оттяжка и возврат. Shift+Scroll и горизонтальный трекпад
// — ровно один шаг на событие, события во время анимации глотаются;
// вертикальное колесо скроллит страницу. Захват указателя в полёте
// перебивает переход и подхватывает ленту с текущей позиции; клик по
// карточке ничего не делает (как у референса). Конечная лента: набор
// позиций покоя — k×step, последняя прижимает правый край стрипа к
// правому краю контентной колонки. prefers-reduced-motion — переходы
// мгновенные.
type Card = {
  title: string;
  text: string;
  textSmall: boolean;
  img: StaticImageData;
  alt: string;
  // Геометрия иллюстрации из макета: ширина; отступ иллюстрации от низа
  // карточки задаёт флаг imgBottomSub — 50px по макету (в разметке это
  // mb-[50px] на планшете-мобайле и desk:mb-[70px] на десктопе), 0 —
  // иллюстрация прижата к низу.
  imgWidth: number;
  imgBottomSub: number;
};

const CARDS: Card[] = [
  {
    title: "Отмечайте оплату аренды",
    text: "Покажем, сколько дней до оплаты и вовремя напомним",
    textSmall: true,
    img: rentalCalendar,
    alt: "Экран оплаты аренды в Рентли",
    imgWidth: 240,
    imgBottomSub: 50,
  },
  {
    title: "Напомним, если платеж просрочится",
    text: "Если вовремя не отметить оплату, платеж станет просроченным",
    textSmall: false,
    img: rentalOverdue,
    alt: "Экран просроченного платежа в Рентли",
    imgWidth: 240,
    imgBottomSub: 50,
  },
  {
    title: "Отслеживайте сроки договора",
    text: "Напомним, когда договор будет подходить к концу",
    textSmall: true,
    img: rentalContract,
    alt: "Экран договора аренды в Рентли",
    imgWidth: 240,
    imgBottomSub: 50,
  },
  {
    title: "Получайте отчет об итогах аренды",
    text: "После завершения, покажем прибыль за период аренды",
    textSmall: false,
    img: rentalReport,
    alt: "Экран отчета по аренде в Рентли",
    imgWidth: 380,
    imgBottomSub: 0,
  },
  {
    title: "Добавляйте контакты арендаторов",
    text: "Контакты арендаторов в одном месте",
    textSmall: false,
    img: rentalContacts,
    alt: "Экран контактов арендаторов в Рентли",
    imgWidth: 320,
    imgBottomSub: 0,
  },
  {
    title: "Возвращайтесь к прошлым арендам",
    text: "История аренд сохраняется, к ней можно вернуться в любой момент",
    textSmall: false,
    img: rentalHistory,
    alt: "Экран истории аренд в Рентли",
    imgWidth: 380,
    imgBottomSub: 0,
  },
];

const N = CARDS.length;

// Длительность переходов — замер секции acquiring Яндекса: 360ms, на
// её мобильном брейкпоинте (<720px) — 460ms.
const STEP_MS = 360;
const STEP_MS_NARROW = 460;
const NARROW_MAX = 720;
// Затухание протяжки за краем: замер Яндекса — сильный жест уводит
// полосу за край примерно на 80px при сырой протяжке ~230px.
const RUBBER = 0.35;
// Порог броска по скорости, px/мс (канон showcase.tsx).
const FLICK_V = 0.5;

// Геометрия ярусов — зеркало CSS: карточки и gap в классах article,
// padding-left дорожки в rentals-carousel.module.css. Колонка ПК —
// 1000px по центру; на планшете-мобиле контент — поле 24px с обоих
// краёв.
const METRICS = {
  desk: { card: 380, gap: 20, column: 1000 },
  touch: { card: 320, gap: 12 },
} as const;
type Tier = keyof typeof METRICS;

function tierOf(): Tier {
  return window.matchMedia("(min-width: 1200px)").matches ? "desk" : "touch";
}

// Позиции покоя: карточка k прижата левым краем к контентной колонке
// (anchor + k×step); последняя позиция прижимает ПРАВЫЙ край стрипа к
// правому краю колонки (maxP = ширина стрипа − ширина колонки) — шаг
// длину стрипа нацело не делит, как у референса.
function model(tier: Tier, vw: number) {
  const m = METRICS[tier];
  const step = m.card + m.gap;
  const stripW = N * m.card + (N - 1) * m.gap;
  const columnW = tier === "desk" ? METRICS.desk.column : vw - 48;
  const anchor = tier === "desk" ? (vw - METRICS.desk.column) / 2 : 24;
  const maxP = Math.max(stripW - columnW, 0);
  const pos: number[] = [];
  for (let k = 0; k * step < maxP - 0.5; k += 1) {
    pos.push(k * step);
  }
  if (pos.length === 0 || maxP - (pos[pos.length - 1] ?? 0) > 0.5) {
    pos.push(maxP);
  }
  return { step, anchor, pos, maxP };
}

// Позиция покоя, ближайшая к p, — по модели, не по живому лэйауту.
function nearestIndex(p: number, pos: readonly number[]): number {
  let best = 0;
  let bestDist = Math.abs((pos[0] ?? 0) - p);
  for (let i = 1; i < pos.length; i += 1) {
    const d = Math.abs((pos[i] ?? 0) - p);
    if (d < bestDist) {
      bestDist = d;
      best = i;
    }
  }
  return best;
}

// Мутабельное состояние ленты — создается в mount-эффекте и меняется
// только в обработчиках (канон refs лендинга: никакого доступа в рендере)
type EngineState = {
  p: number;
  anim: { from: number; to: number; t0: number; dur: number } | null;
  raf: number;
  dragging: boolean;
  dragMoved: boolean;
  pointerId: number | null;
  dragStartX: number;
  dragStartP: number;
  dragSamples: Array<{ t: number; x: number }>;
};

export function RentalsCarousel() {
  const viewportRef = useRef<HTMLDivElement>(null);
  const trackRef = useRef<HTMLDivElement>(null);
  const stateRef = useRef<EngineState | null>(null);
  const reducedRef = useRef(false);
  // animateTo ↔ step образуют цикл (шаг запускает анимацию) — разводим
  // последними ссылками.
  const animateToRef = useRef<(to: number) => void>(() => {});
  const stepRef = useRef<(dir: number) => void>(() => {});

  // Источник истины — st.p; один проход отрисовки кладёт её в дорожку.
  const render = useCallback(() => {
    const track = trackRef.current;
    const st = stateRef.current;
    if (!track || !st) {
      return;
    }
    track.style.transform = `translate3d(${(-st.p).toFixed(2)}px,0,0)`;
  }, []);

  const frameLoop = useCallback(() => {
    const st = stateRef.current;
    if (!st) {
      return;
    }
    const stepFn = (t: number) => {
      const a = st.anim;
      if (!a) {
        return;
      }
      const x = a.dur <= 0 ? 1 : Math.min(Math.max((t - a.t0) / a.dur, 0), 1);
      st.p = a.from + (a.to - a.from) * easeOut(x);
      render();
      if (x < 1) {
        st.raf = requestAnimationFrame(stepFn);
      } else {
        st.anim = null;
      }
    };
    st.raf = requestAnimationFrame(stepFn);
  }, [render]);

  // Единственная анимация движения: фиксированная длительность ease-out
  // до точной позиции покоя, независимо от дистанции.
  const animateTo = useCallback(
    (to: number) => {
      const st = stateRef.current;
      if (!st) {
        return;
      }
      if (Math.abs(st.p - to) < 0.5) {
        st.anim = null;
        st.p = to;
        render();
        return;
      }
      const narrow = window.matchMedia(`(max-width: ${NARROW_MAX - 1}px)`).matches;
      st.anim = {
        from: st.p,
        to,
        t0: performance.now(),
        dur: reducedRef.current ? 0 : narrow ? STEP_MS_NARROW : STEP_MS,
      };
      cancelAnimationFrame(st.raf);
      frameLoop();
    },
    [frameLoop, render],
  );

  // Один шаг в направлении dir: соседняя позиция покоя от ближайшей.
  const step = useCallback((dir: number) => {
    const vp = viewportRef.current;
    const st = stateRef.current;
    if (!vp || !st) {
      return;
    }
    const { pos } = model(tierOf(), vp.clientWidth);
    const cur = nearestIndex(st.p, pos);
    const target = Math.min(Math.max(cur + dir, 0), pos.length - 1);
    animateToRef.current(pos[target] ?? st.p);
  }, []);

  useEffect(() => {
    animateToRef.current = animateTo;
    stepRef.current = step;
  }, [animateTo, step]);

  useEffect(() => {
    const rm = window.matchMedia("(prefers-reduced-motion: reduce)");
    reducedRef.current = rm.matches;
    const onRmChange = () => {
      reducedRef.current = rm.matches;
    };
    rm.addEventListener("change", onRmChange);
    return () => {
      rm.removeEventListener("change", onRmChange);
    };
  }, []);

  // Стартовая позиция (первый вид макета — p=0 без JS) и пересборка на
  // ресайз: ближайшая карточка встаёт ровно на свою позицию покоя.
  useEffect(() => {
    const vp = viewportRef.current;
    if (!vp) {
      return;
    }
    stateRef.current ??= {
      p: 0,
      anim: null,
      raf: 0,
      dragging: false,
      dragMoved: false,
      pointerId: null,
      dragStartX: 0,
      dragStartP: 0,
      dragSamples: [],
    };
    const measure = () => {
      const st = stateRef.current;
      if (!st) {
        return;
      }
      const { pos } = model(tierOf(), vp.clientWidth);
      st.anim = null;
      cancelAnimationFrame(st.raf);
      st.p = pos[nearestIndex(st.p, pos)] ?? 0;
      render();
    };
    measure();
    let resizeRaf = 0;
    const onResize = () => {
      cancelAnimationFrame(resizeRaf);
      resizeRaf = requestAnimationFrame(() => {
        const st = stateRef.current;
        if (!st || st.dragging) {
          return;
        }
        measure();
      });
    };
    window.addEventListener("resize", onResize);
    return () => {
      window.removeEventListener("resize", onResize);
      cancelAnimationFrame(resizeRaf);
      const st = stateRef.current;
      if (st) {
        cancelAnimationFrame(st.raf);
        st.anim = null;
      }
    };
  }, [render]);

  // Drag мышью и тачем — 1:1 за указателем, без инерции; отпуск — докат
  // к ближайшей позиции, быстрый бросок добавляет одну по направлению.
  // Захват в полёте перебивает переход (канон Яндекса); клик по
  // карточке ничего не делает.
  useEffect(() => {
    const vp = viewportRef.current;
    if (!vp) {
      return;
    }
    const st = () => stateRef.current;

    const onPointerDown = (e: PointerEvent) => {
      if (e.pointerType === "mouse" && e.button !== 0) {
        return;
      }
      const s0 = st();
      if (!s0) {
        return;
      }
      s0.dragging = true;
      s0.dragMoved = false;
      s0.pointerId = e.pointerId;
      s0.dragStartX = e.clientX;
      s0.dragStartP = s0.p;
      s0.dragSamples = [{ t: performance.now(), x: e.clientX }];
      s0.anim = null; // drag перехватывает движение
      cancelAnimationFrame(s0.raf);
      vp.classList.add(s.dragging ?? "");
      vp.setPointerCapture(e.pointerId);
    };

    const onPointerMove = (e: PointerEvent) => {
      const s1 = st();
      if (!s1 || !s1.dragging || e.pointerId !== s1.pointerId) {
        return;
      }
      const dx = e.clientX - s1.dragStartX;
      if (Math.abs(dx) > 5) {
        s1.dragMoved = true;
      }
      const { maxP } = model(tierOf(), vp.clientWidth);
      const raw = s1.dragStartP - dx;
      // резиновый край: за пределами позиций полоса следует с затуханием
      s1.p =
        raw < 0 ? raw * RUBBER : raw > maxP ? maxP + (raw - maxP) * RUBBER : raw;
      render();
      s1.dragSamples.push({ t: performance.now(), x: e.clientX });
      if (s1.dragSamples.length > 6) {
        s1.dragSamples.shift();
      }
    };

    const release = (e: PointerEvent, flick: boolean) => {
      const s2 = st();
      if (!s2 || !s2.dragging || e.pointerId !== s2.pointerId) {
        return;
      }
      s2.dragging = false;
      s2.pointerId = null;
      vp.classList.remove(s.dragging ?? "");
      if (!s2.dragMoved) {
        return; // клик без движения — ничего не делает
      }
      const { pos } = model(tierOf(), vp.clientWidth);
      let best = nearestIndex(s2.p, pos);
      if (flick) {
        // скорость по окну ~100мс: последний сэмпл к моменту up почти
        // всегда стоит на месте, оконная скорость ловит бросок даже при
        // остановившемся указателе
        const now = performance.now();
        const windowStart = now - 100;
        const last = s2.dragSamples.at(-1);
        const ref = s2.dragSamples.find((smp) => smp.t >= windowStart);
        if (last && ref && last.t >= windowStart) {
          const dt = Math.max(now - ref.t, 1);
          const vP = -(e.clientX - ref.x) / dt; // скорость p, px/мс
          // бросок двигает ленту на одну карточку, только если тягой
          // порог половины шага не взят (иначе ближайшая уже следующая)
          if (
            Math.abs(vP) > FLICK_V &&
            best === nearestIndex(s2.dragStartP, pos)
          ) {
            best = Math.min(Math.max(best + Math.sign(vP), 0), pos.length - 1);
          }
        }
      }
      animateToRef.current(pos[best] ?? s2.p);
    };

    const onPointerUp = (e: PointerEvent) => release(e, true);
    const onPointerCancel = (e: PointerEvent) => release(e, false);

    vp.addEventListener("pointerdown", onPointerDown);
    vp.addEventListener("pointermove", onPointerMove);
    vp.addEventListener("pointerup", onPointerUp);
    vp.addEventListener("pointercancel", onPointerCancel);
    return () => {
      vp.removeEventListener("pointerdown", onPointerDown);
      vp.removeEventListener("pointermove", onPointerMove);
      vp.removeEventListener("pointerup", onPointerUp);
      vp.removeEventListener("pointercancel", onPointerCancel);
    };
  }, [render]);

  // Shift+Scroll и горизонтальный трекпад — ровно один шаг на событие;
  // события во время анимации или drag глотаются (не в очередь) — серия
  // подряд даёт один шаг, как у референса. Вертикальное колесо не наша
  // ось — скроллит страницу.
  useEffect(() => {
    const vp = viewportRef.current;
    if (!vp) {
      return;
    }
    const onWheel = (e: WheelEvent) => {
      if (Math.abs(e.deltaX) <= Math.abs(e.deltaY)) {
        return;
      }
      e.preventDefault();
      const st = stateRef.current;
      if (!st || st.anim || st.dragging) {
        return;
      }
      stepRef.current(e.deltaX > 0 ? 1 : -1);
    };
    vp.addEventListener("wheel", onWheel, { passive: false });
    return () => {
      vp.removeEventListener("wheel", onWheel);
    };
  }, []);

  return (
    <div ref={viewportRef} className={s.viewport}>
      <div ref={trackRef} className={s.track}>
        {CARDS.map((card) => (
          <article
            key={card.title}
            className="relative h-[500px] w-[320px] shrink-0 overflow-clip rounded-[32px] bg-surface px-6 pt-12 desk:h-[550px] desk:w-[380px] desk:rounded-[40px] desk:px-10 desk:pt-[52px]"
          >
            <div className="mx-auto flex w-full max-w-[220px] flex-col items-center gap-2 text-center desk:gap-3 desk:max-w-[300px]">
              <h3 className="text-[22px] font-medium leading-[26px] desk:text-[28px] desk:leading-8">
                {card.title}
              </h3>
              <p
                className={`text-s leading-5 text-gray-2 ${
                  card.textSmall
                    ? "desk:text-xs desk:leading-[18px]"
                    : "desk:text-r desk:leading-[22px]"
                }`}
              >
                {card.text}
              </p>
            </div>
            <Image
              src={card.img}
              alt={card.alt}
              width={card.imgWidth}
              height={card.imgWidth}
              sizes="(min-width: 1200px) 380px, 320px"
              draggable={false}
              className={`absolute bottom-0 left-1/2 h-auto -translate-x-1/2 ${
                card.imgBottomSub ? "mb-[50px] desk:mb-[70px]" : "mb-0"
              }`}
              style={{ maxWidth: card.imgWidth }}
            />
          </article>
        ))}
      </div>
    </div>
  );
}
