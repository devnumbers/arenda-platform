"use client";

import Image, { type StaticImageData } from "next/image";
import { useCallback, useEffect, useRef, useState } from "react";
import Swiper, { type Swiper as SwiperType } from "swiper";
import { Controller, EffectCoverflow, Mousewheel } from "swiper/modules";
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
// 3005-78445 (планшет), 3008-79114 (мобила). Движок — ядро Swiper,
// всё движение библиотечное (решение владельца 26.10 после лагов
// версии с самописным морфом --k на setTranslate; референс — «Опыт
// партнёров» yandex.ru/id/business, живой замер 26.10): блок собран
// на эффекте coverflow (rotate 0, depth, scale, stretch, slideShadows
// false) с speed 600 и Mousewheel forceToAxis — у нас то же самое.
//
// Морф боковых считает сам coverflow: scale = 1 − (1−0.8)·|дистанция|
// в слотах (сосед ровно 0.8, как в макетах; дальний план 0.6 — на всех
// ярусах за клипом/шторкой, решение владельца 26.10). Геометрия покоя
// прежняя: слайды лежат в слотах полной ширины (spaceBetween 0), а
// stretch = зазор макета − 0.1·ширины (внутрь, отрицательный) ставит
// боковую (×0.8) ровно на 0.9·ширины + зазор от центральной. Ни одного
// самописного обработчика движения: никаких setTranslate-пайплайнов,
// rAF и записи --k — ядро пишет transform слайдов и
// transition-duration само, в полёте интерполирует композитор.
//
// Тайминг — плоский CSS ease @ 600ms (замер 26.10: computed
// transition-timing-function обёртки Яндекса в полёте = ease, подгонка
// по 60 кадрам RMS 0.070 против 0.164 у кривой 24.10 — прежний «замер»
// cubic-bezier(0.33,1,0.68,1) живой анимацией не подтверждается;
// поправка зафиксирована в DESIGN.md §3). Кривая и свойство перехода
// слайдов живут в CSS-модуле — coverflow пишет слайдам только инлайн
// transform и transition-duration.
//
// Бесконечность — 3 копии набора с тихой пересадкой slideTo(±N, 0)
// на покое (renormalizeAll). Нативный loop: true невозможен и на
// Swiper 14.3.0 — воспроизведено живой приёмкой 26.10: петля
// переставляет DOM-слайды в каждой ленте независимо (у лент разные
// шаги 920/421px и контейнеры → разные buffers loopFix), Controller
// сопоставляет ленты позициями сеток и перестановки не передаёт —
// пары «фото↔плашка» перекрещиваются сразу после инициализации
// (photos realIndex 2 при plates realIndex 0). Запрет из DESIGN.md
// §3 подтверждён; пересадка на покое — единственный не-библиотечный
// ход, невидимый (0мс по обеим лентам, только из состояния покоя).
//
// Две ленты — два ядра Swiper, связаны штатным модулем Controller
// (by: 'slide', кольцо в обе стороны): сплайн snapGrid→snapGrid на
// равномерных сетках копий — точный аффинный маппинг; драг 1:1,
// флик-бросок, снап, резинка, синхронный перелёт ведомой — ядро.
// Колесо — модуль Mousewheel (forceToAxis, eventsTarget на зоне
// секции): Shift+Scroll и горизонтальный трекпад листают (события
// в полёте ядро глотает — guard !animating), вертикальное колесо
// скроллит страницу. Клик-в-центр — slideToClickedSlide. Автоплея и
// стрелок клавиатуры нет (решение владельца 24.10); Enter/Space по
// фокусу — slideTo активного слайда (DOM статичен, копии не
// переставляются). Ярусы — по ширине окна (как CSS-медиазапросы
// модуля): на смене яруса пересчитывается только stretch; breakpoints
// Swiper'а не подходят — меряют контейнер, а плашечный контейнер
// уже окна.

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
const SPEED_MS = 600; // любой программный переход — как у Яндекса (замер 26.10)

// Флик-бросок (таб/моб, решение владельца 27.10 «свайп легче»): быстрый
// короткий бросок листает на один шаг. Гейты распознавания броска —
// см. flickRelease в эффекте инициализации; критерий скорости — само
// окно FLICK_MS (бросок быстрее 300мс), отдельной скорости не нужно.
const FLICK_MS = 300; // дольше — это тяга, релизит ratio-ветка
const FLICK_MIN_PX = 30; // короче (в translate ядра) — микросдвиг у тапа
const FLICK_MAX_SHARE = 0.5; // дальше полшага — тяга, релизит ratio-ветка

// Метрики центральных состояний по ярусам — зеркало CSS-переменных
// showcase.module.css. Боковое состояние = центральное × 0.8 — его
// считает coverflow (scale 0.8), внутренних арифметик больше нет.
const METRICS = {
  desk: { card: 1000, gap: 20, plate: 450, plateGap: 16 },
  tab: { card: 500, gap: 12, plate: 400, plateGap: 12 },
  mob: { card: 300, gap: 12, plate: 240, plateGap: 8 },
} as const;
type Tier = keyof typeof METRICS;
type Metrics = (typeof METRICS)[Tier];

// stretch подтягивает боковой слайд (×0.8) из слота полной ширины на
// позицию макета: центр боковой = 0.9·ширины + зазор от центра
// центральной → сдвиг внутрь на 0.1·ширины − зазор (у coverflow
// сдвиг умножается на знак дистанции — правый сосед получает −stretch).
const photoStretchOf = (m: Metrics) => 0.1 * m.card - m.gap;
const plateStretchOf = (m: Metrics) => 0.1 * m.plate - m.plateGap;

const coverflowOf = (stretch: number) => ({
  rotate: 0,
  stretch,
  depth: 0,
  scale: 0.8,
  modifier: 1,
  slideShadows: false,
});

// Порог перелиста тягой — доля шага (longSwipesRatio ядра), мерится в
// translate ядра: на таб/моб 0.15 (решение владельца 27.10 «свайп
// легче»: 75px на планшете, 45px на мобиле вместо полшага 250/150px),
// на ПК 0.5 — релиз по ближайшему снапу (решение 26.10: там колесо и
// клики по боковым, жалоб нет).
const releaseRatioOf = (t: Tier) => (t === "desk" ? 0.5 : 0.15);

export function Showcase() {
  const [mounted, setMounted] = useState(false);
  const stripElRef = useRef<HTMLDivElement>(null); // контейнер ленты плашек
  const photoElRef = useRef<HTMLDivElement>(null); // контейнер фотоленты
  const photoSwRef = useRef<SwiperType | null>(null);
  const platesSwRef = useRef<SwiperType | null>(null);
  const resizePendingRef = useRef(false); // ресайз случился, пока юзер держал ленту

  // Клавиатурная активация: Enter/Space ведут ленту к слайду (DOM
  // статичен — копии не переставляются, индекс рендера валиден всегда).
  const onPlateKeyDown = useCallback((g: number, e: React.KeyboardEvent) => {
    if (e.key !== "Enter" && e.key !== " ") return;
    e.preventDefault();
    const qs = platesSwRef.current;
    if (qs && !qs.destroyed) qs.slideTo(g);
  }, []);

  // Тихая пересадка обеих лент в среднюю копию — только на покое.
  // Позиция — ближайший снап по translate (контроллер может вести
  // ведомую и мимо её активного индекса). Пересадка на N слотов меняет
  // translate ровно на шаг копии; контроллер зеркалит её ведомой — тоже
  // ровно на её N слотов (диапазоны аффинны, дрейфа нет), так что
  // логическое соответствие лент не расходится. Повторный вход после
  // пересадки уже в средней копии — второй вызов no-op. Пока палец на
  // ленте или переход в полёте, пересадка запрещена: при захвате в
  // полёте ядро синхронно диспетчит синтетический transitionend (гася
  // animating), и его JS-translate — ещё целевой, не визуальный;
  // пересадка по нему телепортировала бы ленту из-под пальца. Догоняет
  // пересадку transitionEnd посадочного перехода после релиза. Здесь же
  // дожимается отложенный ресайз (resizePending): update() на покое
  // перестраивает сетки до пересадки.
  const renormalizeAll = useCallback(() => {
    const ps = photoSwRef.current;
    const qs = platesSwRef.current;
    if (!ps || ps.destroyed || !qs || qs.destroyed) return;
    const touched = (sw: SwiperType) => sw.touchEventsData.isTouched === true;
    if (touched(ps) || touched(qs) || ps.animating || qs.animating) return;
    if (resizePendingRef.current) {
      resizePendingRef.current = false;
      ps.update();
      qs.update();
    }
    // запас в 2 слота с каждого края: у самого края копии сосед с той
    // стороны уже пуст, а подготовка к переходу занимает до слота
    const RENORM_MARGIN = 2;
    for (const sw of [ps, qs]) {
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

  // Инициализация обеих лент, геометрия и слушатели ярусов.
  useEffect(() => {
    const photoEl = photoElRef.current;
    const stripEl = stripElRef.current;
    if (!photoEl || !stripEl) return;
    // Ярусы — по ширине окна (как CSS-медиазапросы модуля), не
    // контейнера
    const mmDesk = window.matchMedia("(min-width: 1200px)");
    const mmTab = window.matchMedia("(min-width: 481px)");
    const tierOf = (): Tier => (mmDesk.matches ? "desk" : mmTab.matches ? "tab" : "mob");
    let tier = tierOf();
    const m = METRICS[tier];

    // Канон «захват в полёте подхватывает ленту с текущей позиции»:
    // гасим активный переход в момент нажатия, синхронно перенося
    // JS-позицию в визуальную — иначе на первом move ядро убивает
    // CSS-переход, и тот на кадр вспыхивает целевой позицией полёта
    // (рывок под пальцем, замер 26.10). Тот же паттерн, что у ядра
    // в freeMode.onTouchStart.
    const killFlightOnGrab = (sw: SwiperType) => {
      if (!sw.animating) return;
      sw.setTranslate(sw.getTranslate());
      sw.setTransition(0);
    };

    // Флик-бросок: без самописного движения — в touchEnd, ДО релиз-ветки
    // ядра (emit «touchEnd» стоит в начале onTouchEnd), по признакам
    // броска подменяем longSwipesMs на время одного релиза: ядро уходит
    // в shortSwipes-ветку (слайдит на шаг от интервала, где отпустили
    // палец), параметр возвращает микро-задача. Медленные и длинные тяги
    // остаются в ratio-ветке ближайшего снапа: фикс «перелёта на шаг»
    // 26.10 живёт именно там (дефолтная shortSwipes-ветка слайдила от
    // интервала мимо снапа), поэтому бросок длиннее полшага фликом не
    // считается — ровно-на-шаг перетаскашка доезжает до ratio-ветки и
    // садится на ближайший снап, как принято 26.10. Скорость броска —
    // само окно FLICK_MS: за 300мс дольше 30px (в translate ядра,
    // touchRatio сжимает путь пальца) протащить «медленно» нельзя.
    const flickRelease = (sw: SwiperType) => {
      if (tier === "desk") return;
      const { startTranslate, currentTranslate, touchStartTime } = sw.touchEventsData;
      if (
        typeof startTranslate !== "number" ||
        typeof currentTranslate !== "number" ||
        typeof touchStartTime !== "number"
      ) {
        return;
      }
      const duration = Date.now() - touchStartTime; // часы ядра = Date.now()
      const diff = Math.abs(currentTranslate - startTranslate);
      const step = sw.slidesSizesGrid[0] ?? 0;
      if (
        duration <= 0 ||
        duration >= FLICK_MS ||
        diff < FLICK_MIN_PX ||
        (step > 0 && diff >= FLICK_MAX_SHARE * step)
      ) {
        return;
      }
      sw.params.longSwipesMs = Number.MAX_SAFE_INTEGER;
      queueMicrotask(() => {
        if (!sw.destroyed) sw.params.longSwipesMs = 1;
      });
    };

    // window-resize: обновляем геометрию обеих лент сами (у ядра
    // updateOnWindowResize выключен параметром). Пока юзер держит
    // ленту — только флаг: ядерный update() в этот момент телепортировал
    // бы ленту под пальцем, а ведомая через кольцо контроллеров
    // ответным телепортом дёргала бы мастеровую (замер 26.10); отложенный
    // вызов догоняет renormalizeAll'ом на первой остановке, а релиз без
    // полёта — таймером в touchEnd (transitionEnd там не придёт).
    const applyResizeUpdate = () => {
      const ps = photoSwRef.current;
      const qs = platesSwRef.current;
      if (!ps || ps.destroyed || !qs || qs.destroyed) return;
      const touched = (sw: SwiperType) => sw.touchEventsData.isTouched === true;
      if (touched(ps) || touched(qs) || ps.animating || qs.animating) {
        resizePendingRef.current = true;
        return;
      }
      ps.update();
      qs.update();
      renormalizeAll();
    };
    const applyResizeDeferred = () => {
      window.setTimeout(applyResizeUpdate, SPEED_MS + 100);
    };

    const plates = new Swiper(stripEl, {
      modules: [Controller, EffectCoverflow],
      effect: "coverflow",
      coverflowEffect: coverflowOf(plateStretchOf(m)),
      initialSlide: INITIAL,
      speed: SPEED_MS,
      slidesPerView: "auto",
      centeredSlides: true,
      spaceBetween: 0,
      // релиз тягой — ratio-ветка ядра: longSwipesMs: 1 отправляет в
      // неё любой релиз; порог — releaseRatioOf (решение владельца
      // 26.10, на 27.10 понижен на таб/моб, см. releaseRatioOf).
      // shortSwipes-ветка открывается только гейтами флика (flickRelease):
      // без гейтов она считает prev по границе интервала сетки, и
      // перетаскашка ровно на шаг улетала на шаг дальше (баг 26.10)
      longSwipes: true,
      longSwipesMs: 1,
      longSwipesRatio: releaseRatioOf(tier),
      // догруз картинки внутри контейнера (дефолт lazyPreload: true)
      // зовёт update() со мгновенным slideTo(*, 0) — телепорт ленты
      // под пальцем; картинки секции грузятся eagerly, слушатель не нужен
      lazyPreload: false,
      // window-resize обновляет геометрию наш хендлер (ниже): ядерный
      // update() на ресайзе телепортировал бы ленту под удержанным
      // пальцем и через кольцо контроллеров дёргал вторую ленту
      updateOnWindowResize: false,
      preventClicks: true,
      // Плашки — drag-поверхность: без сужения списка драг, начатый на
      // поверхности сфокусированной кнопки (mousedown её фокусирует),
      // глотается guard'ом ядра (e.target === activeElement и матчится
      // focusableElements → data.isMoved = true до чтения startTranslate),
      // и первый же обработанный move считает позицию от хвоста
      // touchEventsData прошлой тяги — лента телепортирует на разницу
      // одним кадром («секции проскакивают», приёмка 27.10). У div-слайдов
      // фотоленты role="button" с 'button' не матчится — там бага нет;
      // сужено в обеих лентах на упрочнение. Развод тапа и драга у ядра
      // свой (allowClick/threshold), список на него не влияет.
      focusableElements: "input, select, textarea",
      // клик по плашке ведёт её копию в центр; фотолента едет следом
      // через контроллер (клик после драга ядро гасит сам)
      slideToClickedSlide: true,
      on: {
        transitionEnd: () => renormalizeAll(),
        touchStart: () => killFlightOnGrab(plates),
        sliderFirstMove: () => stripEl.classList.add(s.dragging ?? ""),
        touchEnd: () => {
          stripEl.classList.remove(s.dragging ?? "");
          flickRelease(plates);
          applyResizeDeferred();
        },
      },
    });
    platesSwRef.current = plates;

    const photos = new Swiper(photoEl, {
      modules: [Controller, EffectCoverflow, Mousewheel],
      effect: "coverflow",
      coverflowEffect: coverflowOf(photoStretchOf(m)),
      initialSlide: INITIAL,
      speed: SPEED_MS,
      slidesPerView: "auto",
      centeredSlides: true,
      spaceBetween: 0,
      // релиз тягой — см. комментарий в конфиге плашек выше
      longSwipes: true,
      longSwipesMs: 1,
      longSwipesRatio: releaseRatioOf(tier),
      // догруз картинки внутри контейнера (дефолт lazyPreload: true)
      // зовёт update() со мгновенным slideTo(*, 0) — телепорт ленты
      // под пальцем; картинки секции грузятся eagerly, слушатель не нужен
      lazyPreload: false,
      // window-resize обновляет геометрию наш хендлер (ниже): ядерный
      // update() на ресайзе телепортировал бы ленту под удержанным
      // пальцем и через кольцо контроллеров дёргал вторую ленту
      updateOnWindowResize: false,
      preventClicks: true,
      // см. комментарий в конфиге плашек выше
      focusableElements: "input, select, textarea",
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
        transitionEnd: () => renormalizeAll(),
        touchStart: () => killFlightOnGrab(photos),
        sliderFirstMove: () => photoEl.classList.add(s.dragging ?? ""),
        touchEnd: () => {
          photoEl.classList.remove(s.dragging ?? "");
          flickRelease(photos);
          applyResizeDeferred();
        },
      },
    });
    photoSwRef.current = photos;
    // Замыкание кольца контроллеров после создания обеих лент: теперь
    // драг любой ленты ведёт другую
    plates.controller.control = photos;

    // Смена яруса: у обеих лент меняются stretch и порог релиза (слоты
    // полной ширины и spaceBetween 0 от яруса не зависят); update()
    // строит сетки заново, пересадка возвращает активную карточку в центр.
    const onTierChange = () => {
      const nextTier = tierOf();
      if (nextTier === tier) return;
      tier = nextTier;
      const nm = METRICS[nextTier];
      for (const [sw, stretch] of [
        [photos, photoStretchOf(nm)],
        [plates, plateStretchOf(nm)],
      ] as const) {
        const cover = sw.params.coverflowEffect;
        if (!cover) continue;
        cover.stretch = stretch;
        sw.params.longSwipesRatio = releaseRatioOf(nextTier);
        sw.update();
        sw.slideTo(sw.activeIndex, 0);
      }
      renormalizeAll();
    };
    mmDesk.addEventListener("change", onTierChange);
    mmTab.addEventListener("change", onTierChange);

    let resizeRaf = 0;
    const onWindowResize = () => {
      if (resizeRaf !== 0) return;
      resizeRaf = requestAnimationFrame(() => {
        resizeRaf = 0;
        applyResizeUpdate();
      });
    };
    window.addEventListener("resize", onWindowResize);

    setMounted(true);
    return () => {
      mmDesk.removeEventListener("change", onTierChange);
      mmTab.removeEventListener("change", onTierChange);
      window.removeEventListener("resize", onWindowResize);
      if (resizeRaf !== 0) cancelAnimationFrame(resizeRaf);
      photoEl.classList.remove(s.dragging ?? "");
      stripEl.classList.remove(s.dragging ?? "");
      // Passive-cleanup React выполняет уже после удаления поддерева из
      // DOM: destroy-хук Mousewheel-модуля пересчитывает eventsTarget
      // через document.querySelector и с селектором канваса получает
      // null — SPA-переход с главной на юрстраницы падал (#1620).
      // Слушатели снимаем с захваченного swiper.el: removeEventListener
      // на отсоединённом элементе безопасен.
      if (photos.params.mousewheel) {
        photos.params.mousewheel.eventsTarget = "container";
      }
      photos.destroy(true, true);
      plates.destroy(true, true);
      if (photoSwRef.current === photos) photoSwRef.current = null;
      if (platesSwRef.current === plates) platesSwRef.current = null;
    };
  }, [renormalizeAll]);

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
                      const near = c === 1;
                      return (
                        <div key={`${c}-${card.key}`} className={`swiper-slide ${s.plateSlide}`}>
                          <button
                            type="button"
                            tabIndex={near ? 0 : -1}
                            aria-hidden={!near}
                            aria-label={`Показать: ${card.name}`}
                            className={s.plate}
                            onKeyDown={(e) => onPlateKeyDown(g, e)}
                          >
                            <Image src={card.avatar} alt="" className={s.plateIcon} sizes="52px" draggable={false} />
                            <span className={s.plateText}>
                              <span className={s.plateName}>{card.name}</span>
                              <span className={s.plateAddress}>{card.address}</span>
                            </span>
                            <Image src={arrowRight} alt="" className={s.plateArrow} sizes="24px" draggable={false} />
                          </button>
                        </div>
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
                        // eagerly все пять: боковые копии переиспользуют те
                        // же URL, ленивость здесь давала только поп-ины и
                        // догрузки посреди жеста (приёмка 26.10)
                        loading="eager"
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
