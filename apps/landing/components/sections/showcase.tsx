"use client";

import Image, { type StaticImageData } from "next/image";
import { useCallback, useEffect, useRef, useState, type CSSProperties } from "react";
import Swiper, { type Swiper as SwiperType } from "swiper";
import { Controller, Mousewheel } from "swiper/modules";
import "swiper/css";
import { LandingLink } from "@/components/button";
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
// Вся жестовая механика — библиотечная, ни одного самописного
// перехватчика событий (решение владельца 24.10 после краха версии
// с нативной петлёй и форвардом тача; референс — «Опыт партнёров»
// yandex.ru/id/business: core Swiper + coverflow, speed 600, боковые
// карточки уменьшены чистым transform'ом, кривая
// cubic-bezier(0.33, 1, 0.68, 1)).
//
// Две ленты — два ядра Swiper, связаны штатным модулем Controller
// (by: 'slide'): сплайн snapGrid→snapGrid на равномерных сетках копий —
// точный аффинный маппинг (шаг целевой ленты = ровно её N слотов на
// каждые N слотов ведущей), контроллер сам ведёт ведомую (translate,
// индексы, классы, transitionEnd) и не зацикливается (byController-guard).
// Фотолента и плашки драгаются каждая сама: drag 1:1, флик-бросок, снап,
// резинка — release-механика ядра; клик-в-центр — slideToClickedSlide
// (только tap: после драга ядро гасит клик сам); Shift+Scroll и
// горизонтальный трекпад — модуль Mousewheel (forceToAxis) на всей зоне
// секции: вертикальное колесо скроллит страницу, листание — из ядра.
// Enter/Space по фокусу: слайд — onKeyDown, плашка — кнопка, чей
// клавиатурный клик (e.detail === 0) ведёт ленту в центр.
//
// Бесконечность — 3 копии набора с тихой пересадкой slideTo(±N, 0) на
// покое. Нативная петля снята тем же решением: slideToLoop через границу
// обнулял realIndex/классы инстанса, серия колеса валила вкладку.
// С копиями у Controller нет дрейфа: диапазоны translate обеих лент —
// ровно (G−1)·шаг, пересадка на N слотов меняет прогресс на N/(G−1) —
// ведомая уезжает ровно на свои N слотов.
//
// Морф — эффект на штатных событиях (как собственные эффекты Swiper'а):
// k = 1 − |p − snapGrid[i]| / шаг, scale(0.8 + 0.2·k) — слайды лежат в
// слотах центрального размера, боковые сжимаются чистым transform'ом;
// ни одного layout-свойства в кадре (первопричина лагов прежнего
// движка, не возвращать). JS пишет целевые значения (setTranslate) и
// длительности (setTransition), в полёте интерполирует композитор.
// spaceBetween отрицательный — зазор макета между центральной и боковой
// (×0.8): gap − 0.1·ширины, шаг слотов = 0.9·ширины + gap (ставит ядро
// inline-маржином). Автоплея и стрелок клавиатуры нет (решение
// владельца 24.10); лейауты DOM статичны — refs стабильны.

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
const G = N * COPIES; // узлов в каждой ленте
const INITIAL = N + 2; // «2-комнатная на Ленина»: средняя копия, карта 2
const SPEED_MS = 600; // любой программный переход — как у Яндекса (замер 24.10)

// Метрики центральных состояний по ярусам — зеркало CSS-переменных
// showcase.module.css. Боковых размеров нет: боковое состояние =
// центральное × 0.8 чистым scale (внутренние размеры макета — стекло,
// шрифты, блюр, радиусы — ровно 0.8 от центральных на всех ярусах).
const METRICS = {
  desk: { card: 1000, gap: 20, plate: 450, plateGap: 16 },
  tab: { card: 500, gap: 12, plate: 400, plateGap: 12 },
  mob: { card: 300, gap: 12, plate: 240, plateGap: 8 },
} as const;
type Tier = keyof typeof METRICS;
type Metrics = (typeof METRICS)[Tier];

// spaceBetween отрицательный: центры соседних слотов встают так, чтобы
// на покое между центральной карточкой и боковой (×0.8) оставался зазор
// макета: spaceBetween = gap − 0.1·ширины, шаг слотов = 0.9·ширины + gap.
const photoSpaceOf = (m: Metrics) => m.gap - 0.1 * m.card;
const plateSpaceOf = (m: Metrics) => m.plateGap - 0.1 * m.plate;
const photoStepOf = (m: Metrics) => 0.9 * m.card + m.gap;
const plateStepOf = (m: Metrics) => 0.9 * m.plate + m.plateGap;

const clamp = (v: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, v));

// Буферы изменённых узлов за кадр — по одному на ленту, без аллокаций
const photoIdx = new Array<number>(G);
const plateIdx = new Array<number>(G);

// Мутабельное состояние — создается в mount-эффекте и меняется только
// в обработчиках (канон refs лендинга: никакого доступа в рендере)
type ShowcaseState = {
  raf: number; // отложенный рендер (коалинг setTranslate в один кадр)
  tier: Tier;
  photoK: number[]; // последние записанные k слотов фотоленты
  plateK: number[]; // …и плашек
};

export function Showcase() {
  const [mounted, setMounted] = useState(false);
  const stripElRef = useRef<HTMLDivElement>(null); // контейнер ленты плашек
  const photoElRef = useRef<HTMLDivElement>(null); // контейнер фотоленты
  const photoSwRef = useRef<SwiperType | null>(null);
  const platesSwRef = useRef<SwiperType | null>(null);
  const stateRef = useRef<ShowcaseState | null>(null);

  // Один проход отрисовки: k по дистанции до позиций покоя обеих лент.
  // Источник истины — translate и snapGrid Swiper'а (кеш, без чтений
  // лэйаута). Стиль меняют только узлы, чей k ушёл за порог; точные 0/1
  // пишутся всегда: хвост ease затухает ниже порога, и без этого на
  // покое залипает k=0.999 (масштаб до 0.4px мимо).
  const writeK = useCallback((sw: SwiperType, step: number, kBuf: number[], idxBuf: number[]) => {
    let changed = 0;
    for (let j = 0; j < G; j += 1) {
      const nk = clamp(1 - Math.abs(-sw.translate - (sw.snapGrid[j] ?? 0)) / step, 0, 1);
      if (nk === 0 || nk === 1 || Math.abs(nk - (kBuf[j] ?? 0)) > 0.002) {
        kBuf[j] = nk;
        idxBuf[changed] = j;
        changed += 1;
      }
    }
    for (let i = 0; i < changed; i += 1) {
      const j = idxBuf[i] ?? 0;
      sw.slides[j]?.style.setProperty("--k", (kBuf[j] ?? 0).toFixed(3));
    }
  }, []);

  const apply = useCallback(() => {
    const st = stateRef.current;
    const ps = photoSwRef.current;
    const qs = platesSwRef.current;
    if (!st) return;
    // флаг снимается до guard'ов по рефам: apply зовётся из rAF-колбэка,
    // где pending-идентификатор ещё записан, — иначе конвейер глохнет
    // после первого же scheduleApply
    st.raf = 0;
    if (!ps || ps.destroyed || !qs || qs.destroyed) return;
    const m = METRICS[st.tier];
    writeK(ps, photoStepOf(m), st.photoK, photoIdx);
    writeK(qs, plateStepOf(m), st.plateK, plateIdx);
  }, [writeK]);

  // Рендер — не чаще кадра: setTranslate прилетает на каждый
  // pointermove (мыши до 1000Гц), виден только последний кадр
  const scheduleApply = useCallback(() => {
    const st = stateRef.current;
    if (!st || st.raf !== 0) return;
    st.raf = requestAnimationFrame(() => apply());
  }, [apply]);

  // Длительности перехода своей ленты: в полёте едут CSS-transition
  // слайдов синхронно с обёрткой (её ставит ядро), 0 — мгновенная
  // запись (драг, ресайз, тихая пересадка). У плашек вторая ступень —
  // 200ms фоновой подложки, список длительностей парный.
  const setDurations = useCallback((sw: SwiperType, duration: number, plates: boolean) => {
    const dur = duration > 0 ? `${duration}ms` : "0ms";
    for (let j = 0; j < G; j += 1) {
      const slide = sw.slides[j];
      if (slide) slide.style.transitionDuration = plates ? `${dur}, 200ms` : dur;
    }
  }, []);

  // Тихая пересадка обеих лент в среднюю копию — только на покое.
  // Позиция — ближайший снап по translate (контроллер может вести
  // ведомую и мимо её активного индекса). Пересадка на N слотов меняет
  // translate ровно на шаг копии; контроллер зеркалит её ведомой — тоже
  // ровно на её N слотов (диапазоны аффинны, дрейфа нет), так что
  // логическое соответствие лент не расходится. Вызов на transitionEnd
  // обеих лент: обычная посадка стреляет в обе (свою и через контроллер
  // — ведомую), повторный вход после пересадки уже в средней копии.
  const renormalizeAll = useCallback(() => {
    // запас в 2 слота с каждого края: у самого края копии сосед с той
    // стороны уже пуст, а подготовка к переходу занимает до слота
    const RENORM_MARGIN = 2;
    for (const sw of [photoSwRef.current, platesSwRef.current]) {
      if (!sw || sw.destroyed) continue;
      let nearest = 0;
      let bestDist = Infinity;
      for (let j = 0; j < sw.snapGrid.length; j += 1) {
        const d = Math.abs(-sw.translate - (sw.snapGrid[j] ?? 0));
        if (d < bestDist) {
          bestDist = d;
          nearest = j;
        }
      }
      let target = nearest;
      while (target < RENORM_MARGIN) target += N;
      while (target > G - 1 - RENORM_MARGIN) target -= N;
      if (target !== nearest) sw.slideTo(target, 0);
    }
  }, []);

  // Инициализация обеих лент, геометрия и слушатели окна.
  useEffect(() => {
    const photoEl = photoElRef.current;
    const stripEl = stripElRef.current;
    if (!photoEl || !stripEl) return;
    const st = (stateRef.current ??= {
      raf: 0,
      tier: "desk",
      photoK: new Array<number>(G).fill(0),
      plateK: new Array<number>(G).fill(0),
    });
    // Ярусы — по ширине окна (как CSS-медиазапросы модуля), не
    // контейнера
    const mmDesk = window.matchMedia("(min-width: 1200px)");
    const mmTab = window.matchMedia("(min-width: 481px)");
    const tierOf = (): Tier => (mmDesk.matches ? "desk" : mmTab.matches ? "tab" : "mob");
    st.tier = tierOf();
    const m = METRICS[st.tier];

    const plates = new Swiper(stripEl, {
      modules: [Controller],
      initialSlide: INITIAL,
      speed: SPEED_MS,
      slidesPerView: "auto",
      centeredSlides: true,
      spaceBetween: plateSpaceOf(m),
      preventClicks: true,
      // клик по плашке ведёт её копию в центр; фотолента едет следом
      // через контроллер (клик после драга ядро гасит сам)
      slideToClickedSlide: true,
      on: {
        setTranslate: () => scheduleApply(),
        setTransition: (swiper, duration) => setDurations(swiper, duration, true),
        transitionEnd: () => renormalizeAll(),
        sliderFirstMove: () => stripEl.classList.add(s.dragging ?? ""),
        touchEnd: () => stripEl.classList.remove(s.dragging ?? ""),
      },
    });
    platesSwRef.current = plates;

    const photos = new Swiper(photoEl, {
      modules: [Controller, Mousewheel],
      initialSlide: INITIAL,
      speed: SPEED_MS,
      slidesPerView: "auto",
      centeredSlides: true,
      // breakpoints Swiper'а меряют контейнер, а ярус канона (DESIGN.md
      // §2) живёт на ширине окна — ставится вручную на смене яруса
      spaceBetween: photoSpaceOf(m),
      preventClicks: true,
      slideToClickedSlide: true,
      // ведёт плашечную ленту (создана выше); плашки ведут фотоленту
      // через plates.controller.control — кольцо замкнуто в обе стороны
      controller: { control: plates, by: "slide" },
      // Shift+Scroll и горизонтальный трекпад листают ленту; вертикальное
      // колесо остаётся странице (forceToAxis); слушатель — на всей зоне
      // секции, чтобы колесо над плашками тоже листало
      mousewheel: {
        enabled: true,
        forceToAxis: true,
        eventsTarget: "[data-carousel-canvas]",
      },
      on: {
        setTranslate: () => scheduleApply(),
        setTransition: (swiper, duration) => setDurations(swiper, duration, false),
        transitionEnd: () => renormalizeAll(),
        sliderFirstMove: () => photoEl.classList.add(s.dragging ?? ""),
        touchEnd: () => photoEl.classList.remove(s.dragging ?? ""),
      },
    });
    photoSwRef.current = photos;
    // Замыкание кольца контроллеров после создания обеих лент: теперь
    // драг любой ленты ведёт другую
    plates.controller.control = photos;

    // Ресайз окна и смены яруса: гриды пересчитывает update() обеих лент,
    // затем точная пересадка активных слайдов в новые снапы и рендер
    const onViewportChange = () => {
      requestAnimationFrame(() => {
        const s1 = stateRef.current;
        if (!s1) return;
        const nextTier = tierOf();
        if (nextTier !== s1.tier) {
          s1.tier = nextTier;
          const nm = METRICS[nextTier];
          for (const [sw, space] of [
            [photoSwRef.current, photoSpaceOf(nm)],
            [platesSwRef.current, plateSpaceOf(nm)],
          ] as const) {
            if (!sw || sw.destroyed) continue;
            sw.params.spaceBetween = space;
            sw.update();
            sw.slideTo(sw.activeIndex, 0);
          }
        }
        scheduleApply();
      });
    };
    window.addEventListener("resize", onViewportChange);
    mmDesk.addEventListener("change", onViewportChange);
    mmTab.addEventListener("change", onViewportChange);

    setMounted(true);
    return () => {
      window.removeEventListener("resize", onViewportChange);
      mmDesk.removeEventListener("change", onViewportChange);
      mmTab.removeEventListener("change", onViewportChange);
      if (st.raf !== 0) {
        cancelAnimationFrame(st.raf);
        st.raf = 0;
      }
      photoEl.classList.remove(s.dragging ?? "");
      stripEl.classList.remove(s.dragging ?? "");
      photos.destroy(true, true);
      plates.destroy(true, true);
      if (photoSwRef.current === photos) photoSwRef.current = null;
      if (platesSwRef.current === plates) platesSwRef.current = null;
    };
  }, [renormalizeAll, scheduleApply, setDurations]);

  // Клавиатурная активация плашки: настоящий клик с e.detail === 0
  // (Enter/Space на кнопке); мышиные и тачевые проходят через
  // slideToClickedSlide ядра и сюда не доходят по смыслу
  const onPlateKeyDown = useCallback(
    (g: number, e: React.KeyboardEvent) => {
      if (e.key !== "Enter" && e.key !== " ") return;
      e.preventDefault();
      const qs = platesSwRef.current;
      if (qs && !qs.destroyed) qs.slideTo(g);
    },
    [],
  );

  return (
    <section id="showcase" className="mt-24 scroll-mt-[88px] desk:mt-[156px] desk:scroll-mt-[104px]">
      <div className="mx-auto flex max-w-[1048px] flex-col items-center gap-8 px-10 desk:max-w-[1000px] desk:gap-14 desk:px-0">
        <div className="w-full">
          <div className="flex w-full flex-col items-center gap-3 text-center desk:gap-6">
            <h2 className="text-[28px] font-semibold leading-8 tab:text-[32px] tab:leading-9 desk:text-h2 desk:leading-[60px]">
              Создайте карточку своей недвижимости
            </h2>
            <p className="text-s leading-5 text-gray-2 desk:text-[28px] desk:leading-8 desk:font-medium">
              Добавьте квартиру, дом, помещение
            </p>
          </div>
        </div>
        <div className="w-full">
          <div
            data-carousel-canvas
            className={`${s.carousel} ${s.zone}`}
            style={{ visibility: mounted ? "visible" : "hidden" }}
          >
            <div className={s.strip}>
              <div ref={stripElRef} className={`swiper swiper-horizontal ${s.plateSwiper}`}>
                <div className="swiper-wrapper">
                  {Array.from({ length: COPIES }, (_, c) =>
                    CARDS.map((card, i) => {
                      const g = c * N + i;
                      return (
                        <button
                          key={`${c}-${card.key}`}
                          type="button"
                          tabIndex={c === 1 ? 0 : -1}
                          aria-hidden={c !== 1}
                          aria-label={`Показать: ${card.name}`}
                          className={`swiper-slide ${s.plate}`}
                          style={{ "--k": g === INITIAL ? 1 : 0 } as CSSProperties}
                          onKeyDown={(e) => onPlateKeyDown(g, e)}
                        >
                          <Image src={card.avatar} alt="" className={s.plateIcon} sizes="52px" draggable={false} />
                          <span className={s.plateText}>
                            <span className={s.plateName}>{card.name}</span>
                            <span className={s.plateAddress}>{card.address}</span>
                          </span>
                          <Image src={arrowRight} alt="" className={s.plateArrow} sizes="24px" draggable={false} />
                        </button>
                      );
                    }),
                  )}
                </div>
              </div>
              <div className={s.fadeStripLeft} />
              <div className={s.fadeStripRight} />
            </div>
            <div className={s.viewport}>
              <div ref={photoElRef} className={`swiper swiper-horizontal ${s.scroller}`}>
                <div className="swiper-wrapper">
                  {Array.from({ length: COPIES }, (_, c) =>
                    CARDS.map((card, i) => {
                      const g = c * N + i;
                      const near = c === 1;
                      return (
                        <div
                          key={`${c}-${card.key}`}
                          role="button"
                          tabIndex={near ? 0 : -1}
                          aria-hidden={!near}
                          aria-label={`Показать: ${card.name}`}
                          className={`swiper-slide ${s.slide}`}
                          style={{ "--k": g === INITIAL ? 1 : 0 } as CSSProperties}
                          onKeyDown={(e) => {
                            if (e.key === "Enter" || e.key === " ") {
                              e.preventDefault();
                              const ps = photoSwRef.current;
                              if (ps && !ps.destroyed) ps.slideTo(g);
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
        </div>
        <LandingLink href="/login">Попробовать</LandingLink>
      </div>
    </section>
  );
}
