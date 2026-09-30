"use client";

import Image, { type StaticImageData } from "next/image";
import { useCallback, useEffect, useRef, useState } from "react";
import { CarouselDots } from "@/components/carousel-dots";
import {
  createSpring,
  nearestCardIndex,
  type Spring,
} from "@/components/carousel-spring";
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
// — планшет/мобайл (TestimonialsCarousel) — макет 2837-152228: 5
//   отзыв-карточек 320×500 (r32, фото на фоне, тёмная стеклянная плашка
//   снизу: цитата 20/24 Medium + автор — аватар 44, имя 16/20, подпись
//   14/18 white/70), зазор 12, точки-кнопки снизу;
// — десктоп (TestimonialsDesktop) — макеты 2814-979/980: слева отзыв
//   активной карточки (колонка 472px: цитата 36/40 + аватар 64 + имя
//   20/24 + подпись 18/22), справа лента голых фото во всю ширину до
//   края окна: активное 338×450, соседние 300×400 (зазор 20, r40,
//   по центру ряда), соседние приглушены до 0.6 (решение владельца —
//   как у аренды), отзыв слева меняется кроссфейдом. Смена активного —
//   «уход назад», как в секции «Опыт наших партнёров» на
//   pay.yandex.ru/business (решение владельца): уходящая карточка не
//   уезжает влево вместе с лентой — остаётся на линии покоя и уходит
//   вглубь (сжатие до 0.6, растворение в ноль), а новая въезжает на её
//   место, дорастая 300→338 и проявляясь 0.6→1.
//
// Механика лент — общая пружинка лент (components/carousel-spring.ts),
// канон карусели «Управляйте арендой»: глайд с пружинкой-перелётом цели,
// докатка после свайпа по простою скролла, CSS-снап выключен,
// overscroll-x contain, prefers-reduced-motion без пружинки и анимаций.
// Уход назад на десктопе привязан к прогрессу
// скролла (пересчёт на каждом скролл-событии), так что одинаково живёт
// на пружинке, свайпе и докатке. Отличие десктопа: позиции покоя арифметические
// — шаг 320 (малая карточка 300 + зазор 20), активная карточка стоит у
// левой линии ленты, у края скролла (последняя видна целиком, до левой
// линии не дотянуться) активна последняя.
type Review = {
  quote: string;
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
    alt: "Квартира в приложении Рентли",
  },
  {
    quote: "Вижу, сколько заработала и сколько потратила. Наконец понимаю свои расходы",
    name: "Елена",
    sub: "Сдает квартиру в ипотеку",
    avatar: reviewAvatar2,
    photo: slide2,
    alt: "Комната в приложении Рентли",
  },
  {
    quote: "Три студии, и я ничего не путаю. Даты, деньги и контакты всегда под рукой",
    name: "Анна",
    sub: "Сдает три студии в Москве",
    avatar: reviewAvatar3,
    photo: slide3,
    alt: "Дом в приложении Рентли",
  },
  {
    quote: "Четыре арендатора, вижу, кто заплатил, а кто нет. Все договоры и платежи под рукой",
    name: "Алексей",
    sub: "Управляет помещениями в новом ЖК",
    avatar: reviewAvatar4,
    photo: slide4,
    alt: "Офис в приложении Рентли",
  },
  {
    quote: "Сдаю комнаты. Теперь не путаю, кто платил, а кому пора напомнить",
    name: "Марина",
    sub: "Сдает комнаты в трехкомнатной квартире в Екатеринбурге",
    avatar: reviewAvatar5,
    photo: slide5,
    alt: "Склад в приложении Рентли",
  },
];

// ---- Планшет/мобайл: отзыв-карточки ----

const TOUCH_INSET = 24;

export function TestimonialsCarousel() {
  // relative на скроллере делает offsetLeft карточек координатами скролла —
  // на них построены глайд и докатка.
  const scroller = useRef<HTMLDivElement>(null);
  const cards = useRef<Array<HTMLElement | null>>([]);
  const reduced = useRef(false);
  const [active, setActive] = useState(0);
  // Глайд и докатка — общая пружинка лент (components/carousel-spring.ts).
  // Собирается в эффекте: её доступители читают ref-ы ленты, а трогать
  // ref-ы при рендере нельзя; скролл-события раньше эффектов не бывают.
  const springRef = useRef<Spring | null>(null);
  useEffect(() => {
    springRef.current = createSpring({
      scroller: () => scroller.current,
      reduced: () => reduced.current,
    });
    return () => {
      springRef.current?.destroy();
      springRef.current = null;
    };
  }, []);

  useEffect(() => {
    const rm = window.matchMedia("(prefers-reduced-motion: reduce)");
    reduced.current = rm.matches;
    // Настройка может поменяться на лету — держим ref актуальным.
    const onRmChange = () => {
      reduced.current = rm.matches;
    };
    rm.addEventListener("change", onRmChange);
    return () => rm.removeEventListener("change", onRmChange);
  }, []);

  const syncActive = useCallback(() => {
    const el = scroller.current;
    if (!el) {
      return;
    }
    const max = el.scrollWidth - el.clientWidth;
    if (max <= 0) {
      return;
    }
    setActive(Math.round((el.scrollLeft / max) * (REVIEWS.length - 1)));
  }, []);

  // Ближайшая к линии покоя (offsetLeft − TOUCH_INSET) карточка.
  const nearestCardLeft = useCallback(() => {
    const el = scroller.current;
    const nodes = cards.current;
    if (!el || nodes.length === 0) {
      return null;
    }
    const target = el.scrollLeft + (nodes[0]?.offsetWidth ?? 0) / 2;
    const card = nodes[nearestCardIndex(nodes, target)];
    if (!card) {
      return null;
    }
    return card.offsetLeft - TOUCH_INSET;
  }, []);

  // Докатка после свайпа: скролл затих — сами тянем полосу к ближайшей
  // карточке пружинкой (CSS-снап ради этого выключен).
  const settle = useCallback(() => {
    const el = scroller.current;
    if (!el || springRef.current?.isGliding()) {
      return;
    }
    // Резинка на краях (iOS): пока полоса за границами — не докатываем,
    // отскок пришлёт новые скролл-события и таймер перезапустится.
    if (
      el.scrollLeft < 0 ||
      el.scrollLeft > el.scrollWidth - el.clientWidth
    ) {
      return;
    }
    const left = nearestCardLeft();
    if (left === null || Math.abs(left - el.scrollLeft) < 2) {
      return;
    }
    springRef.current?.glideTo(left);
  }, [nearestCardLeft]);

  const onScroll = useCallback(() => {
    syncActive();
    springRef.current?.scheduleSettle(settle);
  }, [settle, syncActive]);

  const goTo = useCallback(
    (index: number) => {
      const el = scroller.current;
      const card = cards.current[index];
      if (!el || !card) {
        return;
      }
      springRef.current?.glideTo(card.offsetLeft - TOUCH_INSET);
    },
    [],
  );

  return (
    <div className="w-full">
      <div
        ref={scroller}
        onScroll={onScroll}
        onPointerDown={() => springRef.current?.stopGlide()}
        onWheel={() => springRef.current?.stopGlide()}
        className="relative flex gap-3 hide-scrollbar px-6 pb-2"
      >
        {REVIEWS.map((review, index) => (
          <figure
            key={review.name}
            ref={(el) => {
              cards.current[index] = el;
            }}
            className="relative h-[500px] w-[320px] shrink-0 overflow-clip rounded-[32px]"
          >
            <Image
              src={review.photo}
              alt={review.alt}
              fill
              sizes="320px"
              className="object-cover"
            />
            <figcaption className="absolute inset-x-2 bottom-2 flex flex-col gap-4 rounded-[24px] bg-black/[0.24] p-6 shadow-[inset_0_0_0_1px_rgba(255,255,255,0.32)] backdrop-blur-[16px]">
              <blockquote className="text-m font-medium leading-6 text-white">
                {review.quote}
              </blockquote>
              <div className="flex items-center gap-3">
                <Image
                  src={review.avatar}
                  alt={review.name}
                  width={44}
                  height={44}
                  className="size-11 rounded-full object-cover"
                />
                <div className="flex flex-col gap-0.5">
                  <p className="text-s leading-5 text-white">{review.name}</p>
                  <p className="max-w-[180px] text-xs leading-[18px] text-white/70">
                    {review.sub}
                  </p>
                </div>
              </div>
            </figcaption>
          </figure>
        ))}
        {/* Хвостовой отступ 24px: последняя карточка встаёт на ту же линию,
            что и первая (100% здесь — контент-бокс без паддингов; 12px
            съедает flex-gap перед спейсером). */}
        <div aria-hidden className="w-[calc(100%_-_332px)] shrink-0" />
      </div>
      <div className="mt-6 flex items-center justify-center gap-4">
        <CarouselDots
          count={REVIEWS.length}
          active={active}
          onSelect={goTo}
          labelFor={(i) => `Слайд ${i + 1}`}
        />
      </div>
    </div>
  );
}

// ---- Десктоп: отзыв слева + лента фото справа ----

// Позиция покоя активной карточки: шаг 320 = малая 300 + зазор 20.
const REST_STEP = 320;
// Кроссфейд отзыва слева (таймаут подмены = длительности перехода).
const REVIEW_FADE_MS = 350;

// «Уход назад» уходящей карточки: сжатие до 0.6 и растворение в ноль
// (у Яндекса активный слайд — scale 1.11, ушедший оседает на 0.6).
// Класс покоя активной карточки (ACTIVE_SCALE_CLASS ниже) — округление
// этих чисел до 4 знаков; править вместе.
const ACTIVE_SCALE_X = 338 / 300;
const ACTIVE_SCALE_Y = 450 / 400;
// Покой активной карточки в разметке: строка — статический литерал, чтобы
// Tailwind увидел класс в исходнике и сгенерировал CSS (динамический
// template literal не сканируется). Значения — ACTIVE_SCALE_X/Y выше,
// округлённые до 4 знаков (338/300 → 1.1267, 450/400 → 1.125); править
// вместе с константами.
const ACTIVE_SCALE_CLASS = "scale-[1.1267_1.125]";
const RECESSED_SCALE = 0.6;
const IDLE_OPACITY = 0.6;
// Добор позиции соседей: активная дорастает до 338 при базе 300, поэтому
// все правые стоят на 38 дальше линии шага — зазор 20 сохраняется.
const ACTIVE_EXTRA_PX = 38;
// Рецесс подтормаживает на старте (степень > 1): пружинковый перелёт
// цели на паре пикселей не дёргает только что приземлившуюся карточку.
const RECEDE_EASE = 1.35;

export function TestimonialsDesktop() {
  const scroller = useRef<HTMLDivElement>(null);
  const cards = useRef<Array<HTMLElement | null>>([]);
  const fadeTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  // settle зависит от пружинки (glideTo), а посадка пружинки перепроверяет
  // линию через settle — разрываем цикл ref-ом (назначение в эффекте ниже).
  const settleRef = useRef<() => void>(() => {});
  const reduced = useRef(false);
  const activeRef = useRef(0);
  const [active, setActive] = useState(0);
  const [shown, setShown] = useState(0);
  const [fading, setFading] = useState(false);
  // Глайд и докатка — общая пружинка лент (components/carousel-spring.ts).
  // Собирается в эффекте: её доступители читают ref-ы ленты, а трогать
  // ref-ы при рендере нельзя. Посадка перепроверяет линию покоя: размеры
  // карточек анимируются вслед за активной (поправка, если что-то уехало);
  // под reduced-motion их снимают классы покоя — перепроверка не нужна.
  const springRef = useRef<Spring | null>(null);
  useEffect(() => {
    springRef.current = createSpring({
      scroller: () => scroller.current,
      reduced: () => reduced.current,
      onLand: (scheduleSettle) => {
        if (!reduced.current) {
          scheduleSettle(() => settleRef.current());
        }
      },
    });
    return () => {
      springRef.current?.destroy();
      springRef.current = null;
    };
  }, []);

  useEffect(() => {
    const rm = window.matchMedia("(prefers-reduced-motion: reduce)");
    reduced.current = rm.matches;
    const onRmChange = () => {
      reduced.current = rm.matches;
      if (rm.matches) {
        // Уход назад считается JS-трансформами; под reduce их снимаем —
        // дальше карточками правят классы состояния покоя.
        for (const card of cards.current) {
          if (!card) {
            continue;
          }
          card.style.translate = "";
          card.style.scale = "";
          card.style.opacity = "";
          card.style.zIndex = "";
        }
      }
    };
    rm.addEventListener("change", onRmChange);
    return () => rm.removeEventListener("change", onRmChange);
  }, []);

  useEffect(
    () => () => {
      if (fadeTimer.current) {
        clearTimeout(fadeTimer.current);
      }
    },
    [],
  );

  // Активная карточка — ближайшая к линии покоя 320×i; у края скролла
  // активна последняя (она видна целиком, но до левой линии не дотянуться).
  const activeFor = useCallback((scrollLeft: number) => {
    const el = scroller.current;
    if (!el) {
      return 0;
    }
    const max = el.scrollWidth - el.clientWidth;
    if (max <= 0) {
      return 0;
    }
    if (scrollLeft >= max - 2) {
      return REVIEWS.length - 1;
    }
    return Math.max(
      0,
      Math.min(REVIEWS.length - 1, Math.round(scrollLeft / REST_STEP)),
    );
  }, []);

  const restFor = useCallback((index: number) => {
    const el = scroller.current;
    if (!el) {
      return 0;
    }
    return Math.max(
      0,
      Math.min(REST_STEP * index, el.scrollWidth - el.clientWidth),
    );
  }, []);

  // Уход назад: на каждом скролл-событии пересчитываем сдвиг, масштаб и
  // прозрачность карточек по их прогрессу p относительно линии покоя
  // (шаг ленты). Карточки всегда в базовом размере 300×400, активная
  // дорастает трансформом — лента не рефловит, арифметика покоя неизменна.
  const applyProgress = useCallback(() => {
    const el = scroller.current;
    if (!el || reduced.current) {
      return;
    }
    const s = el.scrollLeft;
    const a = activeRef.current;
    for (let i = 0; i < REVIEWS.length; i += 1) {
      const card = cards.current[i];
      if (!card) {
        continue;
      }
      const p = (s - REST_STEP * i) / REST_STEP;
      let translate = 0;
      let scale = "";
      let opacity = IDLE_OPACITY;
      let z = 0;
      if (p >= 1) {
        // Прошлая: растворилась в ноль за левой линией.
        opacity = 0;
      } else if (p >= 0) {
        // Уходящая: добор 320·p гасит ход ленты — карточка остаётся на
        // линии покоя и уходит назад (сжатие + растворение).
        const e = p ** RECEDE_EASE;
        translate = REST_STEP * p;
        scale = `${ACTIVE_SCALE_X + (RECESSED_SCALE - ACTIVE_SCALE_X) * e} ${
          ACTIVE_SCALE_Y + (RECESSED_SCALE - ACTIVE_SCALE_Y) * e
        }`;
        opacity = 1 - e;
        z = 1;
      } else if (p > -1) {
        // Приближающаяся: доезжает до линии, дорастая и проявляясь. Добор
        // 38·(−p) тает по мере хода и на границе покоя (p = −1) непрерывно
        // переходит в постоянный добор будущих карточек.
        let q = 1 + p;
        if (q < 0) {
          q = 0;
        }
        if (i === a && Math.abs(s - restFor(a)) < 2) {
          // Лента в покое и не дотянула до линии (край скролла) — карточка
          // всё равно активная, показываем её в полный рост.
          q = 1;
        }
        translate = ACTIVE_EXTRA_PX * Math.min(1, -p);
        scale = `${1 + (ACTIVE_SCALE_X - 1) * q} ${1 + (ACTIVE_SCALE_Y - 1) * q}`;
        opacity = IDLE_OPACITY + (1 - IDLE_OPACITY) * q;
        z = 5;
      } else {
        // Будущая в покое: на 38 дальше линии своего шага.
        translate = ACTIVE_EXTRA_PX;
      }
      card.style.translate = `${translate}px 0px`;
      card.style.scale = scale;
      card.style.opacity = String(opacity);
      card.style.zIndex = String(z);
    }
  }, [restFor]);

  // Первый расчёт после гидрации: классы дают состояние покоя, трансформы
  // активной карточки проставляет JS.
  useEffect(() => {
    applyProgress();
  }, [applyProgress]);

  const settle = useCallback(() => {
    const el = scroller.current;
    if (!el || springRef.current?.isGliding()) {
      return;
    }
    if (
      el.scrollLeft < 0 ||
      el.scrollLeft > el.scrollWidth - el.clientWidth
    ) {
      return;
    }
    const left = restFor(activeFor(el.scrollLeft));
    if (Math.abs(left - el.scrollLeft) < 2) {
      return;
    }
    springRef.current?.glideTo(left);
  }, [activeFor, restFor]);

  useEffect(() => {
    settleRef.current = settle;
  }, [settle]);

  const onScroll = useCallback(() => {
    const el = scroller.current;
    if (!el) {
      return;
    }
    const next = activeFor(el.scrollLeft);
    if (next !== activeRef.current) {
      activeRef.current = next;
      setActive(next);
      // Отзыв слева догоняет активную карточку с кроссфейдом: гасим панель,
      // через паузу подставляем новый отзыв и возвращаем прозрачность.
      setFading(true);
      if (fadeTimer.current) {
        clearTimeout(fadeTimer.current);
      }
      fadeTimer.current = setTimeout(() => {
        setShown(next);
        setFading(false);
      }, reduced.current ? 0 : REVIEW_FADE_MS);
    }
    applyProgress();
    springRef.current?.scheduleSettle(settle);
  }, [activeFor, applyProgress, settle]);

  const goTo = useCallback(
    (index: number) => {
      springRef.current?.glideTo(restFor(index));
    },
    [restFor],
  );

  // Отзыв панели всегда в границах REVIEWS — shown приходит из activeFor.
  const review = REVIEWS[shown];
  if (!review) {
    return null;
  }

  return (
    <div>
      <div className="flex gap-14">
        {/* Панель отзыва гаснет и проявляется на месте — без сдвигов. */}
        <div
          className={`w-[472px] shrink-0 transition-opacity duration-[350ms] motion-reduce:transition-none ${
            fading ? "opacity-0" : "opacity-100"
          }`}
        >
          <blockquote className="text-h3 font-medium leading-10">
            {review.quote}
          </blockquote>
          <div className="mt-8 flex items-center gap-4">
            <Image
              src={review.avatar}
              alt={review.name}
              width={64}
              height={64}
              className="size-16 rounded-[100px] object-cover"
            />
            <div className="flex flex-col gap-2">
              <p className="text-m font-medium leading-6">{review.name}</p>
              <p className="text-r text-gray-2">{review.sub}</p>
            </div>
          </div>
        </div>
        {/* Лента уезжает вправо за колонку до края окна (макет 2814-980).
            Высота 450 — высота ряда по макету: активное фото в полный
            рост 450, соседние 400 по центру. */}
        <div
          ref={scroller}
          onScroll={onScroll}
          onPointerDown={() => springRef.current?.stopGlide()}
          onWheel={() => springRef.current?.stopGlide()}
          className="desk:-mr-[calc((100vw_-_1000px)/2)] flex h-[450px] min-w-0 flex-1 items-center gap-5 hide-scrollbar overflow-y-hidden"
        >
          {REVIEWS.map((item, index) => (
            <Image
              key={item.alt}
              ref={(el) => {
                cards.current[index] = el;
              }}
              src={item.photo}
              alt={item.alt}
              width={338}
              height={450}
              sizes="338px"
              className={`h-[400px] w-[300px] shrink-0 origin-left rounded-[40px] object-cover will-change-[translate,scale,opacity] ${
                active === index
                  ? `z-[5] ${ACTIVE_SCALE_CLASS} opacity-100`
                  : "opacity-60"
              }`}
            />
          ))}
          {/* Хвостовой спейсер: достаёт контент до «последнее фото прижато
              слева» — максимум скролла становится ровно 320×4 при любом
              десктопном вьюпорте (100% здесь — контент-бокс скроллера; 320
              = шаг покоя, из них 20px съедает flex-gap перед спейсером). */}
          <div aria-hidden className="w-[calc(100%_-_320px)] shrink-0" />
        </div>
      </div>
      <div className="mt-14 flex items-center justify-center gap-4">
        <CarouselDots
          count={REVIEWS.length}
          active={active}
          onSelect={goTo}
          labelFor={(i) => `Слайд ${i + 1}`}
        />
      </div>
    </div>
  );
}
