"use client";

import Image, { type StaticImageData } from "next/image";
import { useCallback, useEffect, useRef, useState, type CSSProperties } from "react";
import { LandingLink } from "@/components/button";
import { easeOut } from "@/components/carousel-ease";
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
// от дистанции; клик в полёте перебивает переход с текущей позиции
// (канон DESIGN.md §3), wheel в полёте глотается, drag перехватывает
// движение. Shift+Scroll и горизонтальный трекпад —
// скролл по расстоянию пальца: каждые ~220px накопленного пальцевого
// пути = одна карточка, моментум (затухающие дельты) не летает, свайп
// всегда даёт карточку (механика и константы — в блоке WHEEL ниже),
// вертикальное колесо скроллит страницу.
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
// Слой (will-change: transform) на дорожках держится только на время
// движения (паттерн armLayers testimonials-carousel): постоянные слои —
// ~15 фотокарточек 300px + ~15 плашек на страницу впустую, память на
// дешёвых телефонах. Съём таймером с запасом к 360ms перехода; длинный
// жест перезаводит таймер на каждом кадре движения.
const LAYER_MS = 620;

// Ширины состояний [боковая, центральная] для JS-математики — зеркало
// CSS-переменных showcase.module.css. На ПК боковая растёт с шириной
// экрана (clamp зеркалит --side-w в CSS): внешний край боковой карточки
// всегда уходит за границу экрана. D — дистанция морфинга k, равна
// шагу до соседней карточки (cardSide+gap) на всех ярусах: морфинг
// завершается точно в момент прибытия (как у Яндекса), на покое k
// строго 0/1 — бок ровно 80%, центр ровно по центру вьюпорта.
const METRICS = {
  desk: { cardSide: 800, card: 1000, gap: 20, plateSide: 360, plate: 450, plateGap: 16 },
  tab: { cardSide: 500, card: 500, gap: 12, plateSide: 345, plate: 400, plateGap: 12 },
  mob: { cardSide: 300, card: 300, gap: 12, plateSide: 207, plate: 240, plateGap: 8 },
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
  // На ПК боковая растёт с экраном — зеркало --side-w в CSS
  // (clamp(800px, 100vw − 1640px, 960px)): внешний край боковой
  // всегда за границей экрана до вьюпорта ~2920px
  const cardSide =
    tier === 'desk' ? clamp(viewportW - 1640, m.cardSide, 960) : m.cardSide;
  const step = cardSide + m.gap;
  const copyPlain = N * step;
  const snap = (g: number) =>
    Math.floor(g / N) * copyPlain + (g % N) * step + m.card / 2 - viewportW / 2;
  const snapArr = Array.from({ length: G }, (_, g) => snap(g));
  return { m, cardSide, step, copyPlain, snapArr };
}

const clamp = (v: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, v));

// Кеш геометрии ленты: модель и ширина колонки зависят только от яруса
// и ширины окна, обе меняются только через resize (пересборка в measure
// и onViewportChange). Горячий путь — render() каждый кадр полёта и
// pointermove драга (до сотен событий в секунду) — читает кеш без
// clientWidth: чтение лэйаута после записей прошлого кадра форсирует
// sync-layout посреди кадра. Инвариант: refreshModel зовётся до любого
// render (measure на mount, onViewportChange на resize) — рендер при
// пустом кеше no-op. Ширина колонки — функция ширины окна (макс-w +
// паддинги колонки), не шрифтов, поэтому кеш вне resize не протухает.
// Кеш и changedIdx модульные: движок showcase на странице один.
type CachedModel = ReturnType<typeof snapModel> & { zoneW: number };
let cachedModel: CachedModel | null = null;
function refreshModel(tier: Tier, viewportW: number, zoneW: number): CachedModel {
  cachedModel = { ...snapModel(tier, viewportW), zoneW };
  return cachedModel;
}

// Буфер изменённых узлов за кадр — один на модуль, без аллокаций в кадре
const changedIdx = new Array<number>(G);

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
  // wheel-аккумулятор пальцевого расстояния и буфер формы дельт
  wheelAcc: number;
  wheelSign: number;
  wheelBuf: Array<{ t: number; d: number }>;
  lastWheelT: number;
  lastWheelD: number;
  // отложенный рендер драга (коалесинг pointermove в один кадр) и
  // последние записанные транслейты — чтобы не писать одинаковые строки
  renderRaf: number;
  lastTxPhoto: string;
  lastTxStrip: string;
  // таймер снятия will-change слоёв дорожек (armLayers)
  layerTimer: ReturnType<typeof setTimeout> | null;
};

// ── Wheel: расстояние пальца + фильтр моментума ──────────────────────
// Web-API не отличает моментум трекпада от нового жеста пальцем
// (w3c/pointerevents#596), поэтому классифицируем по форме дельт —
// механика fullPage.js (среднее последних событий ≥ среднего окна =
// палец) + всплеск из wheel-gestures (дельта > 2× прошлой = новый
// свайп поверх инерции). Пальцевое расстояние копится в аккумулятор:
// каждые WHEEL_STEP_PX = +1 карточка (решение владельца 02.10 — скролл
// по расстоянию). Моментум расстояние не копит; если палец успел
// набрать меньше карточки — свайп доводится одной карточкой на первом
// моментум-событии. Решение владельца Q1: сильный свайп может нести
// больше одной — пропорционально реально пройденному пальцем.
const WHEEL_STEP_PX = 220; // пальцевого пути на одну карточку
const WHEEL_COMMIT_MIN_PX = 60; // меньше этого за свайп — не считаем жестом
const WHEEL_QUIET_MS = 200; // тишина = новый жест, буфер формы сбрасывается
const WHEEL_JITTER_PX = 2; // суб-пиксельный шум колеса
const WHEEL_BUF_MAX = 80; // глубина буфера формы дельт

// Палец или моментум? Мало данных (начало жеста) — палец; дельта ровно
// растёт или стоит — палец; монотонно затухает — моментум.
function wheelIsFinger(buf: Array<{ t: number; d: number }>, d: number, prevD: number): boolean {
  if (buf.length === 0) return true; // первое событие после тишины
  if (d > 2 * prevD && d > 24) return true; // всплеск = новый свайп
  if (buf.length < 5) return true; // формы ещё нет — считаем пальцем
  const endCount = Math.min(6, Math.floor(buf.length / 2));
  const end = buf.slice(buf.length - endCount);
  const win = buf.slice(Math.max(0, buf.length - 36));
  const avg = (xs: Array<{ d: number }>) => xs.reduce((s, x) => s + x.d, 0) / xs.length;
  return avg(end) >= avg(win); // затухающий хвост даёт end < win
}

export function Showcase() {
  const [mounted, setMounted] = useState(false);
  const zoneRef = useRef<HTMLDivElement>(null); // колонка карусели (плашки + фото)
  const viewportRef = useRef<HTMLDivElement>(null); // окно фотоленты (full-bleed 100vw)
  const photoTrackRef = useRef<HTMLDivElement>(null); // дорожка фото, translate3d(−p)
  const stripRef = useRef<HTMLDivElement>(null); // дорожка плашек, центруется по p
  const slideNodes = useRef<Array<HTMLElement | null>>([]);
  const plateNodes = useRef<Array<HTMLElement | null>>([]);
  const stateRef = useRef<EngineState | null>(null);

  // Слой (will-change: transform) на дорожках живёт только на время
  // движения: постоянный держит слои всей ленты впустую (LAYER_MS выше).
  // Повторный armLayers без записи стиля, если слой уже навешен.
  const armLayers = useCallback(() => {
    const st = stateRef.current;
    if (!st) return;
    for (const el of [photoTrackRef.current, stripRef.current]) {
      if (el && el.style.willChange !== "transform") {
        el.style.willChange = "transform";
      }
    }
    if (st.layerTimer) {
      clearTimeout(st.layerTimer);
    }
    st.layerTimer = setTimeout(() => {
      st.layerTimer = null;
      if (photoTrackRef.current) photoTrackRef.current.style.willChange = "";
      if (stripRef.current) stripRef.current.style.willChange = "";
    }, LAYER_MS);
  }, []);

  // Один проход отрисовки: k по дистанции до позиций покоя, транслейт
  // фотодорожки, центровка ряда плашек. Источник истины — st.p.
  // Геометрия — из кеша (refreshModel): в кадре полёта ни одного чтения
  // лэйаута и ни одной аллокации. Стиль меняют только узлы, чей k ушёл
  // за порог: запись неизменному узлу перерисовывает его стиль зря
  // (setAttribute пересобирает весь инлайн-стиль), setProperty точечнее.
  const render = useCallback(() => {
    const photoTrack = photoTrackRef.current;
    const strip = stripRef.current;
    const zone = zoneRef.current;
    const st = stateRef.current;
    if (!photoTrack || !strip || !zone || !st || !cachedModel) return;
    const { m, step, snapArr, zoneW } = cachedModel;
    const p = st.p;
    const k = st.k;
    let changed = 0;
    for (let g = 0; g < G; g += 1) {
      const nk = clamp(1 - Math.abs(p - (snapArr[g] ?? 0)) / step, 0, 1);
      // Точные 0/1 пишутся всегда: хвост ease-out затухает ниже порога,
      // и без этого на покое залипает k=0.999 (размер до 0.4px мимо),
      // хотя на покое k строго 0/1
      if (nk === 0 || nk === 1 || Math.abs(nk - (k[g] ?? 0)) > 0.002) {
        k[g] = nk;
        changedIdx[changed] = g;
        changed += 1;
      }
    }
    for (let i = 0; i < changed; i += 1) {
      const g = changedIdx[i] ?? 0;
      const kk = (k[g] ?? 0).toFixed(3);
      slideNodes.current[g]?.style.setProperty("--k", kk);
      plateNodes.current[g]?.style.setProperty("--k", kk);
    }
    const moving = st.anim !== null || st.dragging;
    zone.classList.toggle(s.moving ?? "", moving);
    if (moving) {
      armLayers(); // длинный жест перезаводит таймер снятия на каждом кадре
    }
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
    const tx = zoneW / 2 - (gLo * pitch + m.plate / 2) - frac * pitch;
    // Точность транслейтов одинаковая (0.01px): на 120Гц шаг 0.1px у
    // плашек читался как лёгкая ступенька относительно фотодорожки
    const txPhoto = `translate3d(${(-p).toFixed(2)}px,0,0)`;
    if (txPhoto !== st.lastTxPhoto) {
      st.lastTxPhoto = txPhoto;
      photoTrack.style.transform = txPhoto;
    }
    const txStrip = `translateX(${tx.toFixed(2)}px)`;
    if (txStrip !== st.lastTxStrip) {
      st.lastTxStrip = txStrip;
      strip.style.transform = txStrip;
    }
  }, [armLayers]);

  // Рендер драга — не чаще кадра: pointermove может приходить чаще
  // дисплея (мыши до 1000Гц), каждый прямой render() между событиями —
  // лишняя работа стилей до бесполезной (виден только последний).
  const scheduleRender = useCallback(() => {
    const st = stateRef.current;
    if (!st || st.renderRaf !== 0) return;
    st.renderRaf = requestAnimationFrame(() => {
      st.renderRaf = 0;
      render();
    });
  }, [render]);

  // Тихая пересадка в среднюю копию — только на покое (позиции покоя
  // соседних копий совпадают точно, сдвиг невидим).
  const renormalize = useCallback(() => {
    const st = stateRef.current;
    if (!st || !cachedModel) return;
    const { snapArr, copyPlain } = cachedModel;
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

  // wheelStep ↔ animateTo ↔ frameLoop образуют цикл (докатка серии после
  // анимации → шаг → запуск анимации) — разводим последними ссылками.
  const animateToRef = useRef<(to: number) => void>(() => {});
  const wheelStepRef = useRef<(dir: number) => void>(() => {});

  const frameLoop = useCallback(() => {
    const st = stateRef.current;
    if (!st) return;
    const step = function stepFn(t: number) {
      const a = st.anim;
      if (!a) return;
      const x = a.dur <= 0 ? 1 : clamp((t - a.t0) / a.dur, 0, 1);
      // Последний кадр сажает p ровно в to: from+(to-from)*easeOut(1)
      // несёт FP-пыль (посадка мимо снапа на ~1e-12), из-за неё на покое
      // не сходится строго 0/1
      st.p = x < 1 ? a.from + (a.to - a.from) * easeOut(x) : a.to;
      render();
      if (x < 1) {
        st.raf = requestAnimationFrame(stepFn);
      } else {
        st.anim = null;
        renormalize();
        render(); // снимает .moving (фриз блюра) после посадки
        // Непрерывный скролл: расстояние, набранное пальцем во время
        // анимации, продолжает серию — лок на время перехода, не тишина.
        if (st.wheelAcc >= WHEEL_STEP_PX && st.wheelSign !== 0) {
          st.wheelAcc -= WHEEL_STEP_PX;
          wheelStepRef.current(st.wheelSign);
        }
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
      st.anim = { from: st.p, to, t0: performance.now(), dur: STEP_MS };
      cancelAnimationFrame(st.raf);
      frameLoop();
    },
    [frameLoop, render, renormalize],
  );

  // Один wheel-шаг в направлении dir: сосед от ближайшей позиции покоя.
  const wheelStep = useCallback((dir: number) => {
    const st = stateRef.current;
    if (!st || !cachedModel) return;
    const { snapArr } = cachedModel;
    const cur = nearestIndex(st.p, snapArr);
    const g = clamp(cur + dir, 1, G - 2);
    st.wheelSign = dir;
    animateToRef.current(snapArr[g] ?? st.p);
  }, []);

  useEffect(() => {
    animateToRef.current = animateTo;
    wheelStepRef.current = wheelStep;
  }, [animateTo, wheelStep]);

  // Выбор карточки по индексу узла ленты: кликнутая едет в центр одним
  // переходом, сколько бы шагов ни было (кратчайший обход кольца копий).
  // Клик в полёте перебивает переход с текущей позиции (канон DESIGN.md
  // §3 «клик или захват мыши в полёте перебивает переход», animateTo
  // всегда стартует от живого p); drag перехватывает движение раньше.
  const selectNode = useCallback(
    (g: number) => {
      const st = stateRef.current;
      if (!st || st.dragging || !cachedModel) return;
      const { snapArr } = cachedModel;
      const cur = nearestIndex(st.p, snapArr);
      let delta = g - cur;
      if (delta > G / 2) delta -= G;
      if (delta < -G / 2) delta += G;
      const target = clamp(cur + delta, 0, G - 1);
      animateTo(snapArr[target] ?? st.p);
    },
    [animateTo],
  );

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
      wheelAcc: 0,
      wheelSign: 0,
      wheelBuf: [],
      lastWheelT: 0,
      lastWheelD: 0,
      renderRaf: 0,
      lastTxPhoto: "",
      lastTxStrip: "",
      layerTimer: null,
    };
    // Общая часть смены геометрии (mount и resize — единственные точки):
    // сброс кеша записанных транслейтов и пересборка модели
    const rebase = (st0: EngineState) => {
      st0.lastTxPhoto = "";
      st0.lastTxStrip = "";
      return refreshModel(tierOf(), viewport.clientWidth, zoneRef.current?.clientWidth ?? 0);
    };
    const measure = () => {
      const st0 = stateRef.current;
      if (!st0) return;
      st0.anim = null;
      st0.p = rebase(st0).snapArr[INITIAL] ?? 0;
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
        if (!st0) return;
        // Кеш пересобирается всегда — и в драге тоже: следующий переход
        // после отпускания должен считаться уже по новой геометрии
        const { snapArr, copyPlain } = rebase(st0);
        if (st0.dragging) return;
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
        cancelAnimationFrame(st.renderRaf);
        st.renderRaf = 0;
        st.anim = null;
        if (st.layerTimer) {
          clearTimeout(st.layerTimer);
          st.layerTimer = null;
        }
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
      armLayers(); // слой — до первой записи движения
      viewport.setPointerCapture(e.pointerId);
    };

    const onPointerMove = (e: PointerEvent) => {
      const s1 = st();
      if (!s1 || !s1.dragging || e.pointerId !== s1.pointerId) return;
      const dx = e.clientX - s1.dragStartX;
      if (Math.abs(dx) > 5) s1.dragMoved = true;
      const snapArr = cachedModel?.snapArr;
      if (!snapArr) return;
      s1.p = clamp(s1.dragStartP - dx, snapArr[1] ?? 0, snapArr[G - 2] ?? 0);
      scheduleRender();
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
      const snapArr = cachedModel?.snapArr;
      if (!snapArr) return;
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
  }, [animateTo, armLayers, render, selectNode, scheduleRender]);

  // Shift+Scroll и горизонтальный трекпад — по расстоянию пальца:
  // каждые WHEEL_STEP_PX накопленного пальцевого пути = одна карточка
  // (см. блок констант выше). Моментум (затухающие дельты) расстояние
  // не копит и сам не шагает, но недобранную за свайп карточку доводит
  // ровно одной. Вертикальное колесо не наша ось — скроллит страницу.
  useEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport) return;
    const onWheel = (e: WheelEvent) => {
      if (Math.abs(e.deltaX) <= Math.abs(e.deltaY)) return;
      e.preventDefault();
      const st = stateRef.current;
      if (!st) return;
      const d = Math.abs(e.deltaX);
      const now = performance.now();
      if (now - st.lastWheelT > WHEEL_QUIET_MS) st.wheelBuf.length = 0; // тишина = новый жест
      st.lastWheelT = now;
      if (d < WHEEL_JITTER_PX) return;
      const finger = wheelIsFinger(st.wheelBuf, d, st.lastWheelD);
      st.lastWheelD = d;
      st.wheelBuf.push({ t: now, d });
      if (st.wheelBuf.length > WHEEL_BUF_MAX) st.wheelBuf.shift();
      const dir = e.deltaX > 0 ? 1 : -1;
      if (!finger) {
        // моментум: недобранная за свайп карточка доводится ровно одной
        if (!st.anim && !st.dragging && st.wheelAcc >= WHEEL_COMMIT_MIN_PX) {
          st.wheelAcc = 0;
          wheelStepRef.current(dir);
        }
        return;
      }
      // палец: расстояние копится и в полёте — с потолком в одну
      // карточку, серию после посадки продолжает пост-анимационная
      // проверка в frameLoop
      if (st.anim || st.dragging) {
        st.wheelAcc = Math.min(st.wheelAcc + d, WHEEL_STEP_PX);
        return;
      }
      st.wheelAcc += d;
      if (st.wheelAcc >= WHEEL_STEP_PX) {
        st.wheelAcc -= WHEEL_STEP_PX;
        wheelStepRef.current(dir);
      }
    };
    viewport.addEventListener("wheel", onWheel, { passive: false });
    return () => viewport.removeEventListener("wheel", onWheel);
  }, []);

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
            ref={zoneRef}
            data-carousel-canvas
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
                        aria-hidden={c !== 1}
                        tabIndex={c === 1 ? 0 : -1}
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
                          aria-hidden={!near}
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
        </div>
        <LandingLink href="/login">Попробовать</LandingLink>
      </div>
    </section>
  );
}
