"use client";

import Image, { type StaticImageData } from "next/image";
import { useCallback, useEffect, useRef, useState } from "react";
import { TabletStrip } from "@/components/tablet-strip";
import reviewAvatar1 from "@/assets/sections/review-avatar-1.webp";
import reviewAvatar2 from "@/assets/sections/review-avatar-2.webp";
import reviewAvatar3 from "@/assets/sections/review-avatar-3.webp";
import reviewAvatar4 from "@/assets/sections/review-avatar-4.webp";
import reviewAvatar5 from "@/assets/sections/review-avatar-5.webp";
import slide1 from "@/assets/sections/slide-1.webp";
import slide2 from "@/assets/sections/slide-2.webp";
import slide3 from "@/assets/sections/slide-3.webp";
import slide4 from "@/assets/sections/slide-4.webp";
import slide5 from "@/assets/sections/slide-5.webp";

// Карусель «Опыт пользователей», два вида:
// — планшет/мобайл (TestimonialsCarousel) — кадры 3005-78161/3008-79240:
//   5 отзыв-карточек 320×500 (r32, фото на фоне, тёмная стеклянная плашка
//   снизу: цитата 20/24 Medium + автор — аватар 44, имя 16/20, подпись
//   14/18 white/70), зазор 12, поле 24; на мобилке (≤480) цитаты
//   сокращены редакторски (в макете на узлах стоит аннотация «Сократил
//   текст») — поле quoteShort;
// — десктоп (TestimonialsDesktop) — кадр 2967:75932: слева отзыв активной
//   карточки (колонка 472px: цитата 36/40 + аватар 64 + имя 20/24 +
//   подпись 18/22), справа лента голых фото в контейнере с жёстким
//   обрезом под 3 слота (978px): активная 338×450 (растёт вправо от
//   левой линии — origin-left), соседние 300×400 (зазор 20, r40).
//
// Механика — точная копия секции «Опыт наших партнёров»
// pay.yandex.ru/business (решение владельца): управление ТОЛЬКО кликом по
// видимой карточке на всех ярусах (мышь не тянет, колесо и тач не скроллят
// ленту); клик по соседней сдвигает на 1, клик через одну и дальше —
// прыжок сразу на цель одной анимацией; переход 360ms ease-out: входящая
// едет на место активной, дорастая и проявляясь 0.4→1, бывшая активная
// растворяется на месте (сжатие до 0.6, прозрачность в ноль — у Яндекса
// её translateX равен 0 на каждом кадре при любом прыжке), соседние
// приглушены до 0.4; лента бесконечная (после последней снова первая),
// текст левой колонки меняется мгновенно, автоплея/стрелок/точек нет.
//
// Реализация: нативного скролла нет — карточки absolute, каждый слайд
// несёт свой transform по кольцевой позиции относительно активного
// (rel = (i − active + N) mod N): rel 0 — активная, rel N−1 — «стопка»
// растворённых на месте активной, остальные — очередь справа. Слайд,
// выходящий из стопки в очередь, переезжает без анимации transform
// (утилита slide-jump, как loop-перестановка Яндекса) — иначе он
// перелетал бы весь экран на виду; проявление 0→0.4 при этом остаётся
// плавным. Все состояния слайда — один div-узел: смена тега (button↔div)
// пересоздавала бы узел и рвала CSS-transition (поймано приёмкой).
type Review = {
  quote: string;
  quoteShort?: string;
  name: string;
  sub: string;
  avatar: StaticImageData;
  photo: StaticImageData;
  alt: string;
};

const REVIEWS: Review[] = [
  {
    quote: "Приложение заменило мне заметки и Excel",
    name: "Дмитрий",
    sub: "Сдает квартиру уже второй год",
    avatar: reviewAvatar1,
    photo: slide1,
    alt: "Гостиная квартиры в приложении Рентли",
  },
  {
    quote:
      "Вижу, сколько заработала и сколько потратила. Наконец понимаю свои расходы",
    quoteShort: "Вижу, сколько заработала и сколько потратила",
    name: "Елена",
    sub: "Сдает квартиру в ипотеку",
    avatar: reviewAvatar2,
    photo: slide2,
    alt: "Кухня в приложении Рентли",
  },
  {
    quote:
      "Три студии, и я ничего не путаю. Даты, деньги и контакты всегда под рукой",
    quoteShort: "Три студии, и я ничего не путаю",
    name: "Анна",
    sub: "Сдает три студии в Москве",
    avatar: reviewAvatar3,
    photo: slide3,
    alt: "Спальня в приложении Рентли",
  },
  {
    quote:
      "Четыре арендатора, вижу, кто заплатил, а кто нет. Все договоры и платежи под рукой",
    quoteShort: "Четыре арендатора, вижу, кто заплатил, а кто нет",
    name: "Алексей",
    sub: "Управляет помещениями в новом ЖК",
    avatar: reviewAvatar4,
    photo: slide4,
    alt: "Комната с панорамными окнами в приложении Рентли",
  },
  {
    quote: "Сдаю комнаты. Теперь не путаю, кто платил, а кому пора напомнить",
    quoteShort: "Сдаю комнаты. Теперь не путаю, кто платил",
    name: "Марина",
    sub: "Сдает комнаты в трехкомнатной квартире в Екатеринбурге",
    avatar: reviewAvatar5,
    photo: slide5,
    alt: "Кухня с обеденной зоной в приложении Рентли",
  },
];

const COUNT = REVIEWS.length;

// Десктоп плавнее тача (решение владельца): 440ms и мягкий вход/выход —
// утилиты в globals.css (reduce внутри них, тайминг-функция в arbitrary
// классах ненадёжна — Tailwind съедает часть значения).
const DESK_TRANSITION_CLASS = "slide-desk";
const DESK_JUMP_TRANSITION_CLASS = "slide-jump-desk";
// Заморозка растворения держится чуть дольше перехода (растворение должно
// доиграть до конца, невидимая перестановка — после).
const DESK_FREEZE_MS = 480;
// Растворение уходящей карточки: сжатие до 0.6 в ноль (у Яндекса ушедший
// слайд оседает на 0.6 под наезжающим активным).
const RECESSED_SCALE = 0.6;
// Приглушение соседей в очереди (у Яндекса 0.4).
const IDLE_OPACITY = 0.4;

// Хук клик-карусели: активный индекс + кольцевая позиция каждого слайда
// и флаг прыжка (выход из стопки в очередь). Предыдущий активный хранится
// рядом с текущим (одним state) — прыжок считается на рендере без ref-ов.
// frozen — индекс бывшей активной карточки при прыжке дальше соседней:
// она растворяется НА МЕСТЕ (у Яндекса translateX бывшей активной равен 0
// на каждом кадре, двигаются только scale и opacity), а по завершении
// перехода невидимо переставляется за обрез; таймер хранится в ref —
// мутации только в обработчике клика, не на рендере.
function useClickCarousel(count: number, freezeMs: number) {
  const [active, setActiveState] = useState({ index: 0, previous: 0 });
  const [frozen, setFrozen] = useState<number | null>(null);
  // Слайд, только что вышедший из заморозки: его перестановка на конечную
  // позицию (за обрез) обязана быть мгновенной — с обычным transition он
  // уползает через видимую зону с растущей прозрачностью (поймано
  // приёмкой прыжка «через одну»).
  const [thawed, setThawed] = useState<number | null>(null);
  const frozenTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(
    () => () => {
      if (frozenTimer.current) {
        clearTimeout(frozenTimer.current);
      }
    },
    [],
  );

  const setActive = (index: number) => {
    const k = (index - active.index + count) % count;
    if (k >= 2) {
      setFrozen(active.index);
      setThawed(null);
      if (frozenTimer.current) {
        clearTimeout(frozenTimer.current);
      }
      frozenTimer.current = setTimeout(() => {
        setFrozen(null);
        setThawed(active.index);
      }, freezeMs);
    } else {
      setFrozen(null);
      setThawed(null);
    }
    setActiveState((prev) => ({ index, previous: prev.index }));
  };

  const relOf = (index: number, of: number) => (index - of + count) % count;
  const stateOf = (index: number) => ({
    rel: relOf(index, active.index),
    jumping: relOf(index, active.previous) === count - 1 || index === thawed,
    frozen: index === frozen,
  });

  return { active: active.index, setActive, stateOf };
}

function slideStyle(
  rel: number,
  step: number,
  extra: number,
  activeScale: string,
  frozen: boolean,
): React.CSSProperties {
  // Растворение бывшей активной на месте: позиция покоя заморожена,
  // сжатие и гашение анимируются обычным transition (translateX не
  // меняется — карточка не перелетает, см. useClickCarousel).
  if (frozen) {
    return {
      transform: `translateX(0px) scale(${RECESSED_SCALE})`,
      opacity: 0,
      zIndex: -1,
    };
  }
  if (rel === 0) {
    return { transform: `translateX(0px) scale(${activeScale})`, opacity: 1, zIndex: 5 };
  }
  if (rel === COUNT - 1) {
    return {
      transform: `translateX(0px) scale(${RECESSED_SCALE})`,
      opacity: 0,
      zIndex: -1,
    };
  }
  return {
    transform: `translateX(${rel * step + extra}px) scale(1)`,
    opacity: IDLE_OPACITY,
    zIndex: 0,
  };
}

// Кликабельная карточка — div с ролью кнопки: тег обязан быть одним во
// всех состояниях (см. шапку файла), а phrasing-ограничение <button>
// не пускает внутрь blockquote.
function onSlideKeydown(
  setActive: (index: number) => void,
  index: number,
): React.KeyboardEventHandler<HTMLDivElement> {
  return (event) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      setActive(index);
    }
  };
}

// ---- Планшет/мобайл: разводка ярусов ----

// Стеклянная плашка отзыва — общая для обоих тач-ярусов. Разводка цитат
// внутри: на мобилке (≤480) активна короткая (tab:hidden), на планшете —
// полная (tab:block).
function ReviewPlate({ review }: { review: Review }) {
  return (
    <div className="absolute inset-x-2 bottom-2 flex flex-col gap-4 rounded-[24px] bg-black/[0.24] p-6 shadow-[inset_0_0_0_1px_rgba(255,255,255,0.32)] backdrop-blur-[16px]">
      {review.quoteShort ? (
        <>
          <blockquote className="hidden text-m font-medium leading-6 text-white tab:block">
            {review.quote}
          </blockquote>
          <blockquote className="text-m font-medium leading-6 text-white tab:hidden">
            {review.quoteShort}
          </blockquote>
        </>
      ) : (
        <blockquote className="text-m font-medium leading-6 text-white">
          {review.quote}
        </blockquote>
      )}
      <div className="flex items-center gap-3">
        {/* eager: нативный lazy Chromium не стартует на паре аватаров
            очереди/стопки (поймано приёмкой) — вес после оптимизатора
            ~1-2KB на голову, нечего ленить */}
        <Image
          src={review.avatar}
          alt={review.name}
          width={44}
          height={44}
          loading="eager"
          className="size-11 rounded-full object-cover"
        />
        <div className="flex flex-col gap-0.5">
          <p className="text-s leading-5 text-white">{review.name}</p>
          <p className="max-w-[180px] text-xs leading-[18px] text-white/70">
            {review.sub}
          </p>
        </div>
      </div>
    </div>
  );
}

// Планшет (481–1199): полоса на движке лент — логика «Для кого сервис» /
// «Организуйте дела» (решение владельца): полные равные карточки без
// «активной» и приглушения, движение — drag 1:1 пальцем и
// трекпад/Shift+Scroll (шаг), мышь не тянет, снап, флик, резинка; тап по
// карточке ничего не делает.
function TestimonialsStripCard({ review }: { review: Review }) {
  return (
    <figure className="relative h-[500px] w-[320px] shrink-0 overflow-hidden rounded-[32px]">
      <Image
        src={review.photo}
        alt={review.alt}
        fill
        sizes="320px"
        className="object-cover"
      />
      <ReviewPlate review={review} />
    </figure>
  );
}

// ≤480 и SSR-снапшот: лента по образцу мобильной версии блока «Опыт
// наших партнёров» pay.yandex.ru/business (пересъёмка живыми жестами:
// allowTouchMove true, speed 600, effect coverflow). Активная карточка
// по центру окна (scale 1); соседние полосками выглядывают С ОБЕИХ
// сторон (scale 0.98) — прошедшая стоит вплотную слева и на широких
// окнах видна растущей полосой (у Яндекса 75px на 480, 185px на 700 —
// замер по всем мобильным ширинам; никаких захардкоженных «за краем»),
// приглушения нет — на мобиле Яндекса все видимые непрозрачны.
// Управление: свайп 1:1 за пальцем (старт жеста после
// 5px), отпускание — снап на ближайшую 600ms ease; флик по скорости —
// шаг; медленный драг ≥50% слайда — шаг; горизонтальное колесо/трекпад —
// шаг (серия тиков во время анимации глотается); тап по выглядывающей
// переключает, по активной — ничего (ссылок нет); лента бесконечная
// (loop — краёв и резинки нет). Слайды, чей путь при переходе проходит
// сквозь видимую зону с невидимых краёв, переставляются мгновенно
// (slide-jump) — иначе пролетали бы экран. Мышь ленту не тянет — единый
// канон с полосами; вертикальный тач остаётся скроллом страницы
// (touch-action: pan-y).
const MOB_STEP = 332;
const MOB_MS = 600;
const MOB_DRAG_ACTIVATE_PX = 5;
// Порог долгого свайпа — половина слайда (longSwipes Яндекса).
const MOB_LONG_SWIPE_PX = 160;
// Порог флика по скорости пальца (shortSwipes).
const MOB_FLICK_V = 0.5;

// Позиция слайда по кольцевой позиции относительно активного —
// ЗЕРКАЛЬНАЯ модель Яндекса (замер на 480/600/700): предыдущая стоит
// вплотную слева от активной (translate −шаг, scale 0.98) и на широких
// окнах видна растущей полосой (у них 75px на 480, 185px на 700),
// дальние лесенкой уходят за края. Никаких захардкоженных «за левым
// краем»: позиция не зависит от ширины окна.
function mobPosOf(rel: number): number {
  if (rel === 0) {
    return 0;
  }
  if (rel === 1) {
    return MOB_STEP;
  }
  if (rel === COUNT - 1) {
    return -MOB_STEP;
  }
  return MOB_STEP * 2;
}

// Кольцевая позиция и флаг мгновенного прыжка. Единственный невидимый
// перелёт в кольце из 5 — «за левым дальним ↔ за правым дальним»
// (дистанция 2 шага, оба конца за краями): переставляется без анимации,
// иначе пролетает экран. Функции на уровне модуля: вызов в рендере —
// реак-хук react-hooks/refs консервативно запрещает вызовы компонентных
// функций с ref-доступом.
function mobRelOf(index: number, of: number): number {
  return (index - of + COUNT) % COUNT;
}

function mobStateOf(index: number, activeIndex: number, previous: number) {
  const rel = mobRelOf(index, activeIndex);
  const oldPos = mobPosOf(mobRelOf(index, previous));
  const newPos = mobPosOf(rel);
  const jumping = Math.abs(newPos - oldPos) > MOB_STEP * 1.5;
  return { rel, jumping };
}

function TestimonialsMobileCarousel() {
  const [active, setActiveState] = useState({ index: 0, previous: 0 });
  const [animating, setAnimating] = useState(false);
  // Мосты для нативного wheel-листенера (React вешает wheel пассивным —
  // preventDefault внутри onWheel JSX невозможен и кидает ошибку консоли).
  const activeRef = useRef(0);
  const animatingRef = useRef(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const stageRef = useRef<HTMLDivElement>(null);
  const animTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const drag = useRef<{
    id: number;
    startX: number;
    lastX: number;
    lastT: number;
    v: number;
    moved: boolean;
  } | null>(null);

  useEffect(
    () => () => {
      if (animTimer.current) {
        clearTimeout(animTimer.current);
      }
    },
    [],
  );
  useEffect(() => {
    activeRef.current = active.index;
  }, [active.index]);
  useEffect(() => {
    animatingRef.current = animating;
  }, [animating]);

  const goTo = useCallback(
    (index: number) => {
    if (animating) {
      return;
    }
    const next = ((index % COUNT) + COUNT) % COUNT;
    setAnimating(true);
    setActiveState((prev) => ({ index: next, previous: prev.index }));
    if (animTimer.current) {
      clearTimeout(animTimer.current);
    }
    animTimer.current = setTimeout(() => setAnimating(false), MOB_MS);
    },
    [animating],
  );
  const goToRef = useRef<(index: number) => void>(() => {});
  useEffect(() => {
    goToRef.current = goTo;
  }, [goTo, animating, active.index]);
  // Трекпад/горизонтальное колесо — шаг (пассивный React-листенер не
  // может preventDefault: гориз. свайп трекпада уводил бы в историю).
  useEffect(() => {
    const el = rootRef.current;
    if (!el) {
      return;
    }
    const onWheel = (event: WheelEvent) => {
      if (Math.abs(event.deltaX) < 40 || animatingRef.current) {
        return;
      }
      event.preventDefault();
      goToRef.current(activeRef.current + (event.deltaX > 0 ? 1 : -1));
    };
    el.addEventListener("wheel", onWheel, { passive: false });
    return () => {
      el.removeEventListener("wheel", onWheel);
    };
  }, []);

  const onPointerDown = (event: React.PointerEvent) => {
    if (animating || event.pointerType === "mouse") {
      return;
    }
    drag.current = {
      id: event.pointerId,
      startX: event.clientX,
      lastX: event.clientX,
      lastT: event.timeStamp,
      v: 0,
      moved: false,
    };
  };

  const onPointerMove = (event: React.PointerEvent) => {
    const d = drag.current;
    if (!d || event.pointerId !== d.id) {
      return;
    }
    const dx = event.clientX - d.startX;
    if (!d.moved) {
      if (Math.abs(dx) < MOB_DRAG_ACTIVATE_PX) {
        return;
      }
      d.moved = true;
      rootRef.current?.setPointerCapture(event.pointerId);
    }
    d.v = (event.clientX - d.lastX) / Math.max(1, event.timeStamp - d.lastT);
    d.lastX = event.clientX;
    d.lastT = event.timeStamp;
    const stage = stageRef.current;
    if (stage) {
      stage.style.transform = `translate3d(${dx}px, 0, 0)`;
    }
  };

  const endDrag = (event: React.PointerEvent) => {
    const d = drag.current;
    if (!d || event.pointerId !== d.id) {
      return;
    }
    drag.current = null;
    const stage = stageRef.current;
    if (!stage || !d.moved) {
      return;
    }
    const delta = event.clientX - d.startX;
    let target = active.index;
    if (delta <= -MOB_LONG_SWIPE_PX || d.v <= -MOB_FLICK_V) {
      target = active.index + 1;
    } else if (delta >= MOB_LONG_SWIPE_PX || d.v >= MOB_FLICK_V) {
      target = active.index - 1;
    }
    // Снап контейнера к нулю той же кривой и длительности, что у
    // карточек: суммарное движение (перестановка rel + затухание дельты)
    // остаётся непрерывным (см. свайп-модель в шапке файла).
    const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    stage.style.transition = reduced ? "none" : `transform ${MOB_MS}ms ease`;
    stage.style.transform = "translate3d(0, 0, 0)";
    setTimeout(
      () => {
        stage.style.transition = "";
        stage.style.transform = "";
      },
      reduced ? 0 : MOB_MS + 40,
    );
    if (target !== active.index) {
      goTo(target);
    }
  };

  // Кольцевая позиция и мгновенный прыжок считаются mobStateOf (модуль).
  return (
    <div
      ref={rootRef}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={endDrag}
      onPointerCancel={endDrag}
      className="relative h-[500px] w-full touch-pan-y select-none"
    >
      <div ref={stageRef} className="relative h-full w-full will-change-transform">
        <MobileSlides activeIndex={active.index} previous={active.previous} goTo={goTo} />
      </div>
    </div>
  );
}

// Рендер слайдов мобильной ленты — отдельный компонент без ref-доступов:
// react-hooks/refs консервативно запрещает вызовы компонентных функций
// из рендер-выражений (goTo тянет animTimer.current).
function MobileSlides({
  activeIndex,
  previous,
  goTo,
}: {
  activeIndex: number;
  previous: number;
  goTo: (index: number) => void;
}) {
  return REVIEWS.map((review, index) => {
    const { rel, jumping } = mobStateOf(index, activeIndex, previous);
    const transition = jumping ? "slide-jump" : "slide-mob motion-reduce:transition-none";
    const className = `absolute left-1/2 top-0 -ml-[160px] h-[500px] w-[320px] overflow-hidden rounded-[32px] ${transition} will-change-[transform] motion-reduce:transition-none`;
    const x = mobPosOf(rel);
    const scale = Math.abs(rel) === 1 ? 0.98 : "1";
    const style: React.CSSProperties = {
      transform: `translateX(${x}px) scale(${scale})`,
      zIndex: rel === 0 ? 2 : Math.abs(rel) === 1 ? 1 : 0,
    };
    const slide = (
      <>
        <Image
          src={review.photo}
          alt={review.alt}
          fill
          sizes="320px"
          className="object-cover"
        />
        <ReviewPlate review={review} />
      </>
    );

    // Тапабельны обе видимые полоски (у Яндекса slideToClickedSlide по
    // любому видимому слайду): правая — вперёд, левая — назад.
    if (rel !== 1 && rel !== COUNT - 1) {
      return (
        <div
          key={review.name}
          aria-hidden={rel !== 0}
          className={className}
          style={style}
        >
          {slide}
        </div>
      );
    }
    return (
      <div
        key={review.name}
        role="button"
        tabIndex={0}
        onClick={() => goTo(index)}
        onKeyDown={onSlideKeydown(() => goTo(index), index)}
        aria-label={`Показать отзыв — ${review.name}`}
        className={`${className} cursor-pointer`}
        style={style}
      >
        {slide}
      </div>
    );
  });
}

export function TestimonialsCarousel() {
  return (
    // Полоса планшета во всю ширину окна: карточки — прямые дети дорожки
    // (движок меряет шаг 332 и стрип по ним), поле 24px и зазор 12 даёт
    // tablet-strip.module.css; мобилке достаётся своя лента — поля у неё
    // нет (активная центрируется), обёртке они не нужны (иначе на
    // мобилке поле удваивается).
    <TabletStrip
      stripWidth={1648}
      activeChildren={REVIEWS.map((review) => (
        <TestimonialsStripCard key={review.name} review={review} />
      ))}
    >
      <TestimonialsMobileCarousel />
    </TabletStrip>
  );
}

// ---- Десктоп: отзыв слева + лента фото справа ----

// Активная карточка дорастает до 338×450 трансформом от базы 300×400
// (338/300 и 450/400 — два аргумента scale(), строка покоя ниже
// округлена до 4 знаков).
const ACTIVE_SCALE = "1.1267, 1.125";
// Шаг очереди 320 = малая карточка 300 + зазор 20.
const DESK_STEP = 320;
// Добор позиции очереди: активная дорастает до 338 при базе 300 (рост
// вправо — origin-left), соседи стоят на 38 дальше линии шага — зазор 20
// сохраняется.
const DESK_EXTRA = 38;

export function TestimonialsDesktop() {
  const { active, setActive, stateOf } = useClickCarousel(COUNT, DESK_FREEZE_MS);
  const review = REVIEWS[active];
  if (!review) {
    return null;
  }

  return (
    <div className="flex gap-14">
      {/* Отзыв активной карточки меняется мгновенно — как у Яндекса. */}
      <div className="w-[472px] shrink-0">
        <blockquote className="text-h3 font-medium leading-10">
          {review.quote}
        </blockquote>
        <div className="mt-8 flex items-center gap-4">
          <Image
            src={review.avatar}
            alt={review.name}
            width={64}
            height={64}
            loading="eager"
            className="size-16 rounded-[100px] object-cover"
          />
          <div className="flex flex-col gap-2">
            <p className="text-m font-medium leading-6">{review.name}</p>
            <p className="text-r text-gray-2">{review.sub}</p>
          </div>
        </div>
      </div>
      {/* Лента — контейнер с жёстким обрезом ровно под 3 слота (активная
          338 + зазор 20 + две по 300 + зазоры 20 = 978), как у Яндекса
          (их .swiper-container overflow:hidden шириной 760 при любой
          ширине окна): видно всегда максимум 3 карточки, правее — пустота
          до края окна; на узких окнах раньше режет край окна. Высота 450 —
          высота ряда по макету: активное фото в полный рост 450, соседние
          400 по центру (top 25 = половина разницы). overflow-clip, не
          hidden: hidden делает ленту скролл-контейнером — программный
          scrollIntoView (клик по обрезанной карточке) сдвигает ряд. */}
      <div className="relative h-[450px] w-[978px] shrink-0 overflow-clip">
        {REVIEWS.map((item, index) => {
          const { rel, jumping, frozen } = stateOf(index);
          const transition = jumping
            ? DESK_JUMP_TRANSITION_CLASS
            : DESK_TRANSITION_CLASS;
          const className = `absolute left-0 top-[25px] h-[400px] w-[300px] origin-left overflow-hidden rounded-[40px] ${transition} will-change-[transform,opacity] motion-reduce:transition-none`;
          const style = slideStyle(rel, DESK_STEP, DESK_EXTRA, ACTIVE_SCALE, frozen);
          // Кликабельны только видимые позиции — очередь №1 и №2 (№3 за
          // обрезом; у Яндекса за срезом тоже никто не кликает).
          const interactive = (rel === 1 || rel === 2) && !frozen;
          const photo = (
            <Image
              src={item.photo}
              alt={rel === COUNT - 1 ? "" : item.alt}
              fill
              sizes="338px"
              className="object-cover"
            />
          );

          if (rel === COUNT - 1 || frozen) {
            return (
              <div
                key={item.name}
                aria-hidden
                className={`${className} pointer-events-none`}
                style={style}
              >
                {photo}
              </div>
            );
          }
          if (interactive) {
            return (
              <div
                key={item.name}
                role="button"
                tabIndex={0}
                onClick={() => setActive(index)}
                onKeyDown={onSlideKeydown(setActive, index)}
                aria-label={`Показать отзыв — ${item.name}`}
                className={`${className} cursor-pointer`}
                style={style}
              >
                {photo}
              </div>
            );
          }
          return (
            <div key={item.name} className={className} style={style}>
              {photo}
            </div>
          );
        })}
      </div>
    </div>
  );
}
