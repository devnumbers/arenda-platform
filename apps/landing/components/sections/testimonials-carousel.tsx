"use client";

import Image, { type StaticImageData } from "next/image";
import { useEffect, useRef, useState } from "react";
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

// Переход 360ms ease-out — скорость и кривая карусели Яндекса (speed 360).
const SLIDE_TRANSITION_CLASS =
  "transition-[transform,opacity] duration-[360ms] ease-out";
// Десктоп плавнее тача (решение владельца): 440ms и мягкий вход/выход —
// утилиты в globals.css (reduce внутри них, тайминг-функция в arbitrary
// классах ненадёжна — Tailwind съедает часть значения).
const DESK_TRANSITION_CLASS = "slide-desk";
const DESK_JUMP_TRANSITION_CLASS = "slide-jump-desk";
// Заморозка растворения держится чуть дольше перехода (растворение должно
// доиграть до конца, невидимая перестановка — после).
const TOUCH_FREEZE_MS = 400;
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
  const [active, setActiveState] = useState({ current: 0, previous: 0 });
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
    const k = (index - active.current + count) % count;
    if (k >= 2) {
      setFrozen(active.current);
      setThawed(null);
      if (frozenTimer.current) {
        clearTimeout(frozenTimer.current);
      }
      frozenTimer.current = setTimeout(() => {
        setFrozen(null);
        setThawed(active.current);
      }, freezeMs);
    } else {
      setFrozen(null);
      setThawed(null);
    }
    setActiveState((prev) => ({ current: index, previous: prev.current }));
  };

  const relOf = (index: number, of: number) => (index - of + count) % count;
  const stateOf = (index: number) => ({
    rel: relOf(index, active.current),
    jumping: relOf(index, active.previous) === count - 1 || index === thawed,
    frozen: index === frozen,
  });

  return { active: active.current, setActive, stateOf };
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

// ≤480 и SSR-снапшот: клик-карусель Яндекса (тап по выглядывающей,
// растворение, loop) — планшету не принадлежит, см. TestimonialsCarousel.
function TestimonialsClickCarousel() {
  const { setActive, stateOf } = useClickCarousel(COUNT, TOUCH_FREEZE_MS);

  return (
    <div className="relative mx-6 h-[500px]">
      {REVIEWS.map((review, index) => {
        const { rel, jumping, frozen } = stateOf(index);
        // Прыжок актуален только очередным слайдам (см. useClickCarousel).
        const transition = jumping
          ? "slide-jump"
          : `${SLIDE_TRANSITION_CLASS} motion-reduce:transition-none`;
        const className = `absolute left-0 top-0 h-[500px] w-[320px] overflow-hidden rounded-[32px] ${transition} will-change-[transform,opacity] motion-reduce:transition-none`;
        const style = slideStyle(rel, 332, 0, "1", frozen);
        // Кликабельны только видимые позиции: очередь №1 и №2 (№3 и дальше
        // за обрезом/краем окна — клик по невидимой карточке запускал
        // растворение видимой, поймано приёмкой).
        const interactive = (rel === 1 || rel === 2) && !frozen;
        const slide = (
          <>
            <Image
              src={review.photo}
              alt={rel === COUNT - 1 ? "" : review.alt}
              fill
              sizes="320px"
              className="object-cover"
            />
            <ReviewPlate review={review} />
          </>
        );

        if (rel === COUNT - 1 || frozen) {
          return (
            <div
              key={review.name}
              aria-hidden
              className={`${className} pointer-events-none`}
              style={style}
            >
              {slide}
            </div>
          );
        }
        if (interactive) {
          return (
            <div
              key={review.name}
              role="button"
              tabIndex={0}
              onClick={() => setActive(index)}
              onKeyDown={onSlideKeydown(setActive, index)}
              aria-label={`Показать отзыв — ${review.name}`}
              className={`${className} cursor-pointer`}
              style={style}
            >
              {slide}
            </div>
          );
        }
        return (
          <div key={review.name} className={className} style={style}>
            {slide}
          </div>
        );
      })}
    </div>
  );
}

export function TestimonialsCarousel() {
  return (
    // Полоса планшета во всю ширину окна: карточки — прямые дети дорожки
    // (движок меряет шаг 332 и стрип по ним), поле 24px и зазор 12 даёт
    // tablet-strip.module.css; мобилке достаётся прежний корень — поля
    // (mx-6) живут в самой клик-карусели, обёртке они не нужны (иначе
    // на мобилке поле удваивается).
    <TabletStrip
      stripWidth={1648}
      activeChildren={REVIEWS.map((review) => (
        <TestimonialsStripCard key={review.name} review={review} />
      ))}
    >
      <TestimonialsClickCarousel />
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
