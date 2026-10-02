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
// Механика: фотолента — нативный горизонтальный скроллер со snap
// (бесконечность — 3 копии набора с тихой пересадкой на среднюю),
// плашки — синхронный ряд, морфинг размеров через CSS --k.

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
const INITIAL = N + 2; // «2-комнатная на Ленина»: средняя копия (слайды N..2N−1), карта 2
// Ширины состояний [боковая, центральная] для JS-математики — зеркало
// CSS-переменных showcase.module.css; D — дистанция морфинга k (шаг до
// соседней карточки).
const METRICS = {
  desk: { cardSide: 800, card: 1000, gap: 20, plateSide: 360, plate: 450, plateGap: 16, D: 920 },
  tab: { cardSide: 500, card: 500, gap: 12, plateSide: 345, plate: 400, plateGap: 12, D: 512 },
  mob: { cardSide: 300, card: 300, gap: 12, plateSide: 207, plate: 240, plateGap: 8, D: 312 },
} as const;
type Tier = keyof typeof METRICS;

/* Точная модель позиций покоя. В состоянии покоя активная карточка —
   центральная ширина, остальные — боковые, поэтому позиция скролла,
   центрирующая слайд g (копия c = floor(g/N), карта i = g mod N):
   S(g) = c×copyPlain + i×(sideW+gap) + centerW/2 − viewportW/2,
   copyPlain = N×(sideW+gap). Сдвиг на copyPlain переводит S(g)↔S(g±N)
   точно (соседние копии визуально идентичны) — на нём держится
   бесконечность. Все целевые позиции считаются из модели, не из живого
   лэйаута: ширины карточек меняются при морфинге, живые offsetLeft в
   момент старта анимации дают ошибку на (centerW−sideW)/2. */
function snapModel(tier: Tier, viewportW: number) {
  const m = METRICS[tier];
  const step = m.cardSide + m.gap;
  const copyPlain = N * step;
  const snap = (g: number) =>
    Math.floor(g / N) * copyPlain + (g % N) * step + m.card / 2 - viewportW / 2;
  const snapArr = Array.from({ length: N * COPIES }, (_, g) => snap(g));
  return { m, step, copyPlain, snapArr };
}

const clamp = (v: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, v));

function tierOf(): Tier {
  if (window.matchMedia("(min-width: 1200px)").matches) return "desk";
  if (window.matchMedia("(min-width: 481px)").matches) return "tab";
  return "mob";
}

const lerp = (side: number, center: number, k: number) => side + (center - side) * k;

// Мутабельное состояние ленты — создается в mount-эффекте и меняется
// только в обработчиках (канон refs лендинга: никакого доступа в рендере)
type CarouselState = {
  k: number[];
  dragging: boolean;
  dragMoved: boolean;
  dragStartX: number;
  dragStartScroll: number;
  dragSamples: Array<{ t: number; x: number }>;
};

export function Showcase() {
  const [mounted, setMounted] = useState(false);
  const scrollerRef = useRef<HTMLDivElement>(null);
  const stripRef = useRef<HTMLDivElement>(null);
  const zoneRef = useRef<HTMLDivElement>(null);
  const stateRef = useRef<CarouselState | null>(null);

  const applyFrame = useCallback(() => {
    const scroller = scrollerRef.current;
    const strip = stripRef.current;
    const zone = zoneRef.current;
    const state = stateRef.current;
    if (!scroller || !strip || !zone || !state) return;
    const { m, snapArr } = snapModel(tierOf(), scroller.clientWidth);
    const scroll = scroller.scrollLeft;
    const k = state.k;
    let dirty = false;
    for (let g = 0; g < N * COPIES; g++) {
      const nk = clamp(1 - Math.abs(scroll - (snapArr[g] ?? 0)) / m.D, 0, 1);
      if (Math.abs(nk - (k[g] ?? 0)) > 0.002) {
        k[g] = nk;
        dirty = true;
      }
    }
    if (dirty) {
      for (let g = 0; g < N * COPIES; g++) {
        const kk = (k[g] ?? 0).toFixed(3);
        scroller.children[g]?.setAttribute("style", `--k:${kk}`);
        strip.children[g]?.setAttribute("style", `--k:${kk}`);
      }
    }
    // Плашки: транслируем ряд так, чтобы активная (непрерывный индекс)
    // сидела по центру зоны; ширины плашек берём из тех же k.
    const pillW = (i: number) => lerp(m.plateSide, m.plate, k[Math.min(i, N * COPIES - 1)] ?? 0);
    let gLo = 0;
    while (gLo < N * COPIES - 1 && (snapArr[gLo + 1] ?? 0) <= scroll) gLo++;
    const next = snapArr[Math.min(gLo + 1, N * COPIES - 1)] ?? 0;
    const span = Math.max(next - (snapArr[gLo] ?? 0), 1);
    const frac = clamp((scroll - (snapArr[gLo] ?? 0)) / span, 0, 1);
    const activeFloat = gLo + frac;
    let centerOffset = 0;
    for (let j = 0; j < Math.floor(activeFloat); j++) centerOffset += pillW(j) + m.plateGap;
    centerOffset += (activeFloat - Math.floor(activeFloat)) * (pillW(Math.min(Math.floor(activeFloat) + 1, N * COPIES - 1)) + m.plateGap);
    centerOffset += pillW(Math.floor(activeFloat)) / 2;
    const tx = zone.clientWidth / 2 - centerOffset;
    strip.style.transform = `translateX(${tx.toFixed(1)}px)`;
  }, [stateRef]);

  const settle = useCallback(() => {
    const scroller = scrollerRef.current;
    const zone = zoneRef.current;
    const state = stateRef.current;
    if (!scroller || !zone || !state || state.dragging) return;
    const { snapArr, copyPlain } = snapModel(tierOf(), scroller.clientWidth);
    let best = 0;
    let bestDist = Math.abs((snapArr[0] ?? 0) - scroller.scrollLeft);
    for (let g = 1; g < N * COPIES; g++) {
      const d = Math.abs((snapArr[g] ?? 0) - scroller.scrollLeft);
      if (d < bestDist) {
        bestDist = d;
        best = g;
      }
    }
    // нормализация копии: активный слайд держим в средней копии
    while (best < 2) {
      best += N;
      scroller.scrollLeft += copyPlain;
    }
    while (best > N * COPIES - 3) {
      best -= N;
      scroller.scrollLeft -= copyPlain;
    }
    // магнит: доводка до точной позиции покоя (морфинг сместил живые
    // оффсеты, snap оставил ≤ пары сотен пикселей)
    scroller.scrollLeft = snapArr[best] ?? 0;
    applyFrame();
  }, [applyFrame, stateRef]);

  // Короткий шаг ленты от текущего положения: steps = ±1/±2/0. Клик по
  // боковой карточке/плашке знает направление геометрически (левее/правее
  // центра) — на границе набора это даёт правильную сторону обхода
  // бесконечной ленты, nearest-copy логика тут уезжает назад.
  const stepCarousel = useCallback(
    (steps: number, smooth = true) => {
      const scroller = scrollerRef.current;
      if (!scroller || steps === 0) return;
      const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
      const { snapArr } = snapModel(tierOf(), scroller.clientWidth);
      let cur = 0;
      let curDist = Math.abs((snapArr[0] ?? 0) - scroller.scrollLeft);
      for (let g = 1; g < N * COPIES; g++) {
        const d = Math.abs((snapArr[g] ?? 0) - scroller.scrollLeft);
        if (d < curDist) {
          curDist = d;
          cur = g;
        }
      }
      const g = clamp(cur + steps, 2, N * COPIES - 3);
      scroller.scrollTo({
        left: snapArr[g] ?? 0,
        behavior: smooth && !reduced ? "smooth" : "auto",
      });
    },
    [],
  );

  // Стартовая центровка и пересборка базы на смену яруса
  useEffect(() => {
    const scroller = scrollerRef.current;
    if (!scroller) return;
    stateRef.current ??= {
      k: new Array<number>(N * COPIES).fill(0),
      dragging: false,
      dragMoved: false,
      dragStartX: 0,
      dragStartScroll: 0,
      dragSamples: [],
    };
    const measure = () => {
      const st0 = stateRef.current;
      if (!st0) return;
      scroller.scrollLeft = snapModel(tierOf(), scroller.clientWidth).snapArr[INITIAL] ?? 0;
      applyFrame();
    };
    measure();
    setMounted(true);
    let mqDesk = window.matchMedia("(min-width: 1200px)");
    let mqTab = window.matchMedia("(min-width: 481px)");
    const onTierChange = () => {
      const wasDesk = mqDesk.matches;
      const wasWide = mqTab.matches;
      mqDesk = window.matchMedia("(min-width: 1200px)");
      mqTab = window.matchMedia("(min-width: 481px)");
      if (mqDesk.matches !== wasDesk || mqTab.matches !== wasWide) {
        measure();
        settle();
      }
    };
    mqDesk.addEventListener("change", onTierChange);
    mqTab.addEventListener("change", onTierChange);
    return () => {
      mqDesk.removeEventListener("change", onTierChange);
      mqTab.removeEventListener("change", onTierChange);
    };
  }, [applyFrame, settle, stateRef]);

  useEffect(() => {
    const scroller = scrollerRef.current;
    if (!scroller) return;
    let raf = 0;
    const onScroll = () => {
      cancelAnimationFrame(raf);
      raf = requestAnimationFrame(applyFrame);
    };
    const onEnd = () => settle();
    scroller.addEventListener("scroll", onScroll, { passive: true });
    scroller.addEventListener("scrollend", onEnd);
    return () => {
      scroller.removeEventListener("scroll", onScroll);
      scroller.removeEventListener("scrollend", onEnd);
      cancelAnimationFrame(raf);
    };
  }, [applyFrame, settle]);

  // Drag мышью; тач и тачпад скроллят нативно
  useEffect(() => {
    const scroller = scrollerRef.current;
    if (!scroller) return;
    const st = () => stateRef.current;

    const onPointerDown = (e: PointerEvent) => {
      if (e.pointerType !== "mouse" || e.button !== 0) return;
      const s0 = st();
      if (!s0) return;
      s0.dragging = true;
      s0.dragMoved = false;
      s0.dragStartX = e.clientX;
      s0.dragStartScroll = scroller.scrollLeft;
      s0.dragSamples = [{ t: performance.now(), x: e.clientX }];
      scroller.classList.add(s.dragging ?? "");
    };
    const onPointerMove = (e: PointerEvent) => {
      const s1 = st();
      if (!s1 || !s1.dragging) return;
      const dx = e.clientX - s1.dragStartX;
      if (Math.abs(dx) > 5) s1.dragMoved = true;
      scroller.scrollLeft = s1.dragStartScroll - dx;
      s1.dragSamples.push({ t: performance.now(), x: e.clientX });
      if (s1.dragSamples.length > 6) s1.dragSamples.shift();
    };
    const onPointerUp = (e: PointerEvent) => {
      const s2 = st();
      if (!s2 || !s2.dragging) return;
      s2.dragging = false;
      scroller.classList.remove(s.dragging ?? "");
      if (!s2.dragMoved) {
        // клик: pointer capture меняет цель click, хит-тестим вручную
        const el = document.elementFromPoint(e.clientX, e.clientY);
        const host = el?.closest<HTMLElement>("[data-card-idx]");
        if (host) {
          const r = host.getBoundingClientRect();
          const off = r.left + r.width / 2 - window.innerWidth / 2;
          stepCarousel(Math.abs(off) < r.width / 4 ? 0 : Math.sign(off));
        }
        return;
      }
      // инерция: проецируем скорость мыши на соседний слайд
      const last = s2.dragSamples.at(-1);
      if (!last) return;
      const dt = Math.max(performance.now() - last.t, 1);
      const vx = (e.clientX - last.x) / dt; // px/ms, минус — тянем влево
      const { snapArr } = snapModel(tierOf(), scroller.clientWidth);
      const projected = scroller.scrollLeft - vx * 110;
      let best = 0;
      let bestDist = Math.abs((snapArr[0] ?? 0) - projected);
      for (let g = 1; g < N * COPIES; g++) {
        const d = Math.abs((snapArr[g] ?? 0) - projected);
        if (d < bestDist) {
          bestDist = d;
          best = g;
        }
      }
      const cur = snapArr.reduce((a, x, gi) => (Math.abs(x - scroller.scrollLeft) < Math.abs((snapArr[a] ?? 0) - scroller.scrollLeft) ? gi : a), 0);
      const chosen = clamp(best, cur - 1, cur + 1);
      scroller.scrollTo({ left: snapArr[chosen] ?? 0, behavior: "smooth" });
    };
    scroller.addEventListener("pointerdown", onPointerDown);
    window.addEventListener("pointermove", onPointerMove);
    window.addEventListener("pointerup", onPointerUp);
    return () => {
      scroller.removeEventListener("pointerdown", onPointerDown);
      window.removeEventListener("pointermove", onPointerMove);
      window.removeEventListener("pointerup", onPointerUp);
    };
  }, [stepCarousel, stateRef]);

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
                        type="button"
                        data-card-idx={i}
                        aria-label={`Показать: ${card.name}`}
                        className={s.plate}
                        style={{ "--k": g === INITIAL ? 1 : 0 } as CSSProperties}
                        onClick={(e) => {
                          if (stateRef.current?.dragMoved) return;
                          const r = e.currentTarget.getBoundingClientRect();
                          const off = r.left + r.width / 2 - window.innerWidth / 2;
                          stepCarousel(Math.abs(off) < r.width / 4 ? 0 : Math.sign(off));
                        }}
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
              <div ref={scrollerRef} className={s.scroller}>
                {Array.from({ length: COPIES }, (_, c) =>
                  CARDS.map((card, i) => {
                    const g = c * N + i;
                    const near = c === 1;
                    return (
                      <div
                        key={`${c}-${card.key}`}
                        data-card-idx={i}
                        role="button"
                        tabIndex={near ? 0 : -1}
                        aria-label={`Показать: ${card.name}`}
                        className={s.slide}
                        style={{ "--k": g === INITIAL ? 1 : 0 } as CSSProperties}
                        onClick={(e) => {
                          if (stateRef.current?.dragMoved) return;
                          const r = e.currentTarget.getBoundingClientRect();
                          const off = r.left + r.width / 2 - window.innerWidth / 2;
                          stepCarousel(Math.abs(off) < r.width / 4 ? 0 : Math.sign(off));
                        }}
                        onKeyDown={(e) => {
                          if (e.key === "Enter" || e.key === " ") {
                            e.preventDefault();
                            const r = e.currentTarget.getBoundingClientRect();
                            const off = r.left + r.width / 2 - window.innerWidth / 2;
                            stepCarousel(Math.abs(off) < r.width / 4 ? 0 : Math.sign(off));
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
