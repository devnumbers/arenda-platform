"use client";

import Image, { type StaticImageData } from "next/image";
import { useCallback, useEffect, useRef, useState } from "react";
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
//   по центру ряда), при смене активного размеры плавно дорастают,
//   соседние приглушены до 0.6 (решение владельца — как у аренды),
//   отзыв слева меняется кроссфейдом.
//
// Механика лент — канон карусели «Управляйте арендой» (rentals-carousel):
// глайд с пружинкой-перелётом цели, докатка после свайпа по простою
// скролла, CSS-снап выключен, overscroll-x contain, prefers-reduced-motion
// без пружинки и анимаций. Отличие десктопа: позиции покоя арифметические
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

// ---- Общие константы пружинки (канон аренды) ----

// Глайд: длительность растёт с дистанцией, потолок 850мс.
const GLIDE_BASE_MS = 350;
const GLIDE_MAX_MS = 850;
const GLIDE_MS_PER_PX = 0.25;

// Пружинка: лёгкий перелёт цели и упругий возврат.
const OVERSHOOT_MIN_PX = 6;
const OVERSHOOT_MAX_PX = 24;
const OVERSHOOT_RATIO = 0.05;
const OVERSHOOT_SETTLE_MS = 360;
const HOP_MAX_PX = 40; // короче — мягкий переход без перелёта
const HOP_MS = 420;
// Докатка после свайпа: простой скролла, прежде сами тянем к ближайшей.
const SETTLE_IDLE_MS = 140;

// ---- Планшет/мобайл: отзыв-карточки ----

const TOUCH_INSET = 24;

export function TestimonialsCarousel() {
  // relative на скроллере делает offsetLeft карточек координатами скролла —
  // на них построены глайд и докатка.
  const scroller = useRef<HTMLDivElement>(null);
  const cards = useRef<Array<HTMLElement | null>>([]);
  const glideRaf = useRef(0);
  const glideActive = useRef(false);
  const settleTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const reduced = useRef(false);
  const [active, setActive] = useState(0);

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

  useEffect(
    () => () => {
      cancelAnimationFrame(glideRaf.current);
      if (settleTimer.current) {
        clearTimeout(settleTimer.current);
      }
    },
    [],
  );

  const stopGlide = useCallback(() => {
    cancelAnimationFrame(glideRaf.current);
    glideActive.current = false;
  }, []);

  // Пружинка: мягкий разгон-торможение, лёгкий перелёт цели, упругий возврат.
  // Короткие ходы (< HOP_MAX_PX) — просто плавный переход без перелёта.
  const glideTo = useCallback((targetLeft: number) => {
    const el = scroller.current;
    if (!el) {
      return;
    }
    cancelAnimationFrame(glideRaf.current);
    if (settleTimer.current) {
      clearTimeout(settleTimer.current);
      settleTimer.current = null;
    }
    const left = Math.max(
      0,
      Math.min(targetLeft, el.scrollWidth - el.clientWidth),
    );
    glideActive.current = true;
    const finish = () => {
      glideActive.current = false;
    };
    const from = el.scrollLeft;
    const delta = left - from;
    if (reduced.current || Math.abs(delta) < 1) {
      el.scrollLeft = left;
      finish();
      return;
    }
    const easeInOutCubic = (t: number) =>
      t < 0.5 ? 4 * t * t * t : 1 - (-2 * t + 2) ** 3 / 2;
    const easeOutCubic = (t: number) => 1 - (1 - t) ** 3;
    const start = performance.now();
    if (Math.abs(delta) < HOP_MAX_PX) {
      const step = (now: number) => {
        const t = Math.min(1, (now - start) / HOP_MS);
        el.scrollLeft = from + delta * easeInOutCubic(t);
        if (t < 1) {
          glideRaf.current = requestAnimationFrame(step);
        } else {
          finish();
        }
      };
      glideRaf.current = requestAnimationFrame(step);
      return;
    }
    const peak =
      left +
      Math.sign(delta) *
        Math.min(
          OVERSHOOT_MAX_PX,
          Math.max(OVERSHOOT_MIN_PX, Math.abs(delta) * OVERSHOOT_RATIO),
        );
    const mainDuration = Math.min(
      GLIDE_MAX_MS,
      GLIDE_BASE_MS + Math.abs(delta) * GLIDE_MS_PER_PX,
    );
    const step = (now: number) => {
      const t1 = (now - start) / mainDuration;
      if (t1 < 1) {
        el.scrollLeft = from + (peak - from) * easeInOutCubic(Math.max(0, t1));
        glideRaf.current = requestAnimationFrame(step);
        return;
      }
      const t2 = (now - start - mainDuration) / OVERSHOOT_SETTLE_MS;
      el.scrollLeft =
        peak + (left - peak) * easeOutCubic(Math.min(1, Math.max(0, t2)));
      if (t2 < 1) {
        glideRaf.current = requestAnimationFrame(step);
      } else {
        el.scrollLeft = left;
        finish();
      }
    };
    glideRaf.current = requestAnimationFrame(step);
  }, []);

  // Ближайшая к линии покоя (offsetLeft − TOUCH_INSET) карточка.
  const nearestCardLeft = useCallback(() => {
    const el = scroller.current;
    const nodes = cards.current;
    if (!el || nodes.length === 0) {
      return null;
    }
    const target = el.scrollLeft + (nodes[0]?.offsetWidth ?? 0) / 2;
    let best = -1;
    let bestDist = Infinity;
    for (let i = 0; i < nodes.length; i += 1) {
      const card = nodes[i];
      if (!card) {
        continue;
      }
      const dist = Math.abs(card.offsetLeft + card.offsetWidth / 2 - target);
      if (dist < bestDist) {
        bestDist = dist;
        best = i;
      }
    }
    const card = nodes[best];
    if (!card) {
      return null;
    }
    return card.offsetLeft - TOUCH_INSET;
  }, []);

  // Докатка после свайпа: скролл затих — сами тянем полосу к ближайшей
  // карточке пружинкой (CSS-снап ради этого выключен).
  const settle = useCallback(() => {
    const el = scroller.current;
    if (!el || glideActive.current) {
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
    glideTo(left);
  }, [glideTo, nearestCardLeft]);

  const onScroll = useCallback(() => {
    syncActive();
    if (glideActive.current) {
      return;
    }
    if (settleTimer.current) {
      clearTimeout(settleTimer.current);
    }
    settleTimer.current = setTimeout(settle, SETTLE_IDLE_MS);
  }, [settle, syncActive]);

  const goTo = useCallback(
    (index: number) => {
      const el = scroller.current;
      const card = cards.current[index];
      if (!el || !card) {
        return;
      }
      glideTo(card.offsetLeft - TOUCH_INSET);
    },
    [glideTo],
  );

  return (
    <div className="w-full">
      <div
        ref={scroller}
        onScroll={onScroll}
        onPointerDown={stopGlide}
        onWheel={stopGlide}
        className="relative flex gap-3 overflow-x-auto overscroll-x-contain px-6 pb-2 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
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
        {REVIEWS.map((review, index) => (
          <button
            key={review.name}
            type="button"
            aria-label={`Слайд ${index + 1}`}
            aria-current={active === index}
            onClick={() => goTo(index)}
            className={`size-3 cursor-pointer rounded-full transition-colors duration-200 ${
              active === index ? "bg-ink" : "bg-line hover:bg-gray-3"
            }`}
          />
        ))}
      </div>
    </div>
  );
}

// ---- Десктоп: отзыв слева + лента фото справа ----

// Позиция покоя активной карточки: шаг 320 = малая 300 + зазор 20.
const REST_STEP = 320;
// Кроссфейд отзыва слева.
const REVIEW_FADE_MS = 200;

export function TestimonialsDesktop() {
  const scroller = useRef<HTMLDivElement>(null);
  const glideRaf = useRef(0);
  const glideActive = useRef(false);
  const settleTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const fadeTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  // settle зависит от glideTo, а glideTo после посадки перепроверяет линию
  // через settle — разрываем цикл ref-ом (назначение в эффекте ниже).
  const settleRef = useRef<() => void>(() => {});
  const reduced = useRef(false);
  const activeRef = useRef(0);
  const [active, setActive] = useState(0);
  const [shown, setShown] = useState(0);
  const [fading, setFading] = useState(false);

  useEffect(() => {
    const rm = window.matchMedia("(prefers-reduced-motion: reduce)");
    reduced.current = rm.matches;
    const onRmChange = () => {
      reduced.current = rm.matches;
    };
    rm.addEventListener("change", onRmChange);
    return () => rm.removeEventListener("change", onRmChange);
  }, []);

  useEffect(
    () => () => {
      cancelAnimationFrame(glideRaf.current);
      if (settleTimer.current) {
        clearTimeout(settleTimer.current);
      }
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

  const stopGlide = useCallback(() => {
    cancelAnimationFrame(glideRaf.current);
    glideActive.current = false;
  }, []);

  // Пружинка — канон аренды: разгон-торможение, лёгкий перелёт цели,
  // упругий возврат; короткие ходы — мягкий переход без перелёта.
  const glideTo = useCallback((targetLeft: number) => {
    const el = scroller.current;
    if (!el) {
      return;
    }
    cancelAnimationFrame(glideRaf.current);
    if (settleTimer.current) {
      clearTimeout(settleTimer.current);
      settleTimer.current = null;
    }
    const left = Math.max(
      0,
      Math.min(targetLeft, el.scrollWidth - el.clientWidth),
    );
    glideActive.current = true;
    const finish = () => {
      glideActive.current = false;
      // Размеры карточек анимируются вслед за активной — после посадки
      // перепроверяем линию покоя (поправка, если что-то уехало).
      if (!reduced.current) {
        if (settleTimer.current) {
          clearTimeout(settleTimer.current);
        }
        settleTimer.current = setTimeout(() => settleRef.current(), SETTLE_IDLE_MS);
      }
    };
    const from = el.scrollLeft;
    const delta = left - from;
    if (reduced.current || Math.abs(delta) < 1) {
      el.scrollLeft = left;
      finish();
      return;
    }
    const easeInOutCubic = (t: number) =>
      t < 0.5 ? 4 * t * t * t : 1 - (-2 * t + 2) ** 3 / 2;
    const easeOutCubic = (t: number) => 1 - (1 - t) ** 3;
    const start = performance.now();
    if (Math.abs(delta) < HOP_MAX_PX) {
      const step = (now: number) => {
        const t = Math.min(1, (now - start) / HOP_MS);
        el.scrollLeft = from + delta * easeInOutCubic(t);
        if (t < 1) {
          glideRaf.current = requestAnimationFrame(step);
        } else {
          finish();
        }
      };
      glideRaf.current = requestAnimationFrame(step);
      return;
    }
    const peak =
      left +
      Math.sign(delta) *
        Math.min(
          OVERSHOOT_MAX_PX,
          Math.max(OVERSHOOT_MIN_PX, Math.abs(delta) * OVERSHOOT_RATIO),
        );
    const mainDuration = Math.min(
      GLIDE_MAX_MS,
      GLIDE_BASE_MS + Math.abs(delta) * GLIDE_MS_PER_PX,
    );
    const step = (now: number) => {
      const t1 = (now - start) / mainDuration;
      if (t1 < 1) {
        el.scrollLeft = from + (peak - from) * easeInOutCubic(Math.max(0, t1));
        glideRaf.current = requestAnimationFrame(step);
        return;
      }
      const t2 = (now - start - mainDuration) / OVERSHOOT_SETTLE_MS;
      el.scrollLeft =
        peak + (left - peak) * easeOutCubic(Math.min(1, Math.max(0, t2)));
      if (t2 < 1) {
        glideRaf.current = requestAnimationFrame(step);
      } else {
        el.scrollLeft = left;
        finish();
      }
    };
    glideRaf.current = requestAnimationFrame(step);
  }, []);

  const settle = useCallback(() => {
    const el = scroller.current;
    if (!el || glideActive.current) {
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
    glideTo(left);
  }, [activeFor, glideTo, restFor]);

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
    if (glideActive.current) {
      return;
    }
    if (settleTimer.current) {
      clearTimeout(settleTimer.current);
    }
    settleTimer.current = setTimeout(settle, SETTLE_IDLE_MS);
  }, [activeFor, settle]);

  const goTo = useCallback(
    (index: number) => {
      glideTo(restFor(index));
    },
    [glideTo, restFor],
  );

  // Отзыв панели всегда в границах REVIEWS — shown приходит из activeFor.
  const review = REVIEWS[shown];
  if (!review) {
    return null;
  }

  return (
    <div>
      <div className="flex gap-14">
        <div
          className={`w-[472px] shrink-0 transition-[opacity,translate] duration-200 ease-out motion-reduce:transition-none ${
            fading ? "translate-y-1 opacity-0" : "translate-y-0 opacity-100"
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
            Высота фиксированная 450 — высота ряда по макету: без неё секция
            «дышит», пока активное фото сжимается, а новое дорастает. */}
        <div
          ref={scroller}
          onScroll={onScroll}
          onPointerDown={stopGlide}
          onWheel={stopGlide}
          className="desk:-mr-[calc((100vw_-_1000px)/2)] flex h-[450px] min-w-0 flex-1 items-center gap-5 overflow-x-auto overflow-y-hidden overscroll-x-contain [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
        >
          {REVIEWS.map((item, index) => (
            <Image
              key={item.alt}
              src={item.photo}
              alt={item.alt}
              width={338}
              height={450}
              sizes="338px"
              className={`shrink-0 rounded-[40px] object-cover transition-[width,height,opacity] duration-300 ease-out motion-reduce:transition-none ${
                active === index
                  ? "h-[450px] w-[338px] opacity-100"
                  : "h-[400px] w-[300px] opacity-60"
              }`}
            />
          ))}
          {/* Хвостовой спейсер: достаёт контент до «последнее фото прижато
              слева» — максимум скролла становится ровно 320×4 при любом
              десктопном вьюпорте (100% здесь — контент-бокс скроллера; 20px
              съедает flex-gap перед спейсером). */}
          <div aria-hidden className="w-[calc(100%_-_358px)] shrink-0" />
        </div>
      </div>
      <div className="mt-14 flex items-center justify-center gap-4">
        {REVIEWS.map((item, index) => (
          <button
            key={item.name}
            type="button"
            aria-label={`Слайд ${index + 1}`}
            aria-current={active === index}
            onClick={() => goTo(index)}
            className={`size-3 cursor-pointer rounded-full transition-colors duration-200 ${
              active === index ? "bg-ink" : "bg-line hover:bg-gray-3"
            }`}
          />
        ))}
      </div>
    </div>
  );
}
