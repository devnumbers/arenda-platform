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

// Карусель «Опыт пользователей» — макет 2837-152228: 5 отзыв-карточек
// 320×500 (r32, фото на фоне, тёмная стеклянная плашка снизу: цитата
// 20/24 Medium + автор — аватар 44, имя 16/20, подпись 14/18 white/70),
// зазор 12, точки-кнопки снизу — на всех брейкпоинтах.
//
// Механика — канон карусели «Управляйте арендой» (rentals-carousel.tsx):
// десктоп — «полка» (активная карточка по центру окна, по краям призрачное
// место; масштаб и приглушение по дистанции до центра в rAF); докатка после
// свайпа и клик по точке — пружинка с лёгким перелётом цели и упругим
// возвратом (CSS-снап выключен везде, докатку ведём сами по простою
// скролла). Планшет/мобайл — та же логика: отступ 24px до первой и после
// последней (хвостовой спейсер даёт последней встать на ту же линию),
// тач-моментум нативный, «полки» на тачах нет. overscroll-x contain не
// даёт краевому свайпу дёргать навигацию. prefers-reduced-motion — без
// пружинки и «полки», скролл мгновенный. Отличие от аренды: карточка одна
// на все брейкпоинты (320×500) и зазор 12 без десктопных 20 — по узлу.
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

const DESK_QUERY = "(min-width: 1200px)";

// Отступ первой/последней карточки на планшете-мобайле: линия покоя
// карточки = её offsetLeft минус этот паддинг скроллера.
const TOUCH_INSET = 24;

// «Полка»: масштаб и приглушение карточки на краю окна (у активной — 1).
const SHELF_SCALE_FAR = 0.93;
const SHELF_OPACITY_FAR = 0.6;

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

export function TestimonialsCarousel() {
  // relative на скроллере делает offsetLeft карточек координатами скролла —
  // на них построены и глайд, и «полка».
  const scroller = useRef<HTMLDivElement>(null);
  const cards = useRef<Array<HTMLElement | null>>([]);
  const frame = useRef(0);
  const glideRaf = useRef(0);
  const glideActive = useRef(false);
  const settleTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const reduced = useRef(false);
  const [desk, setDesk] = useState(false);
  const [active, setActive] = useState(0);
  const activeRef = useRef(0);

  useEffect(() => {
    const rm = window.matchMedia("(prefers-reduced-motion: reduce)");
    reduced.current = rm.matches;
    // Настройка может поменяться на лету — держим ref актуальным.
    const onRmChange = () => {
      reduced.current = rm.matches;
    };
    rm.addEventListener("change", onRmChange);
    const mq = window.matchMedia(DESK_QUERY);
    const update = () => setDesk(mq.matches);
    update();
    mq.addEventListener("change", update);
    return () => {
      rm.removeEventListener("change", onRmChange);
      mq.removeEventListener("change", update);
    };
  }, []);

  // Один проход на кадр: читаем геометрию стрипа, ведём активную точку, на
  // десктопе рисуем «полку» (масштаб и прозрачность по дистанции до центра).
  const sync = useCallback(() => {
    const el = scroller.current;
    const nodes = cards.current;
    if (!el || nodes.length === 0) {
      return;
    }
    const target =
      el.scrollLeft +
      (desk ? el.clientWidth / 2 : (nodes[0]?.offsetWidth ?? 0) / 2);
    let best = 0;
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
      if (desk && !reduced.current) {
        const p = Math.max(0, 1 - dist / (el.clientWidth / 2));
        card.style.transform = `scale(${(
          SHELF_SCALE_FAR +
          (1 - SHELF_SCALE_FAR) * p
        ).toFixed(4)})`;
        card.style.opacity = (
          SHELF_OPACITY_FAR +
          (1 - SHELF_OPACITY_FAR) * p
        ).toFixed(3);
      } else {
        // Карусель или reduced-motion: инлайн-стили «полки» не нужны.
        card.style.transform = "";
        card.style.opacity = "";
      }
    }
    if (activeRef.current !== best) {
      activeRef.current = best;
      setActive(best);
    }
  }, [desk]);

  // Активная точка и «полка» до первого скролла и при смене брейкпоинта.
  useEffect(() => {
    sync();
  }, [sync]);

  useEffect(() => {
    const onResize = () => {
      cancelAnimationFrame(frame.current);
      frame.current = requestAnimationFrame(sync);
    };
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, [sync]);

  useEffect(
    () => () => {
      cancelAnimationFrame(frame.current);
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

  // Ближайшая к целевой точке карточка: её позиция скролла (центр/левый край).
  const nearestCardLeft = useCallback(() => {
    const el = scroller.current;
    const nodes = cards.current;
    if (!el || nodes.length === 0) {
      return null;
    }
    const target =
      el.scrollLeft +
      (desk ? el.clientWidth / 2 : (nodes[0]?.offsetWidth ?? 0) / 2);
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
    return desk
      ? card.offsetLeft + card.offsetWidth / 2 - el.clientWidth / 2
      : card.offsetLeft - TOUCH_INSET;
  }, [desk]);

  // Докатка после свайпа: скролл затих — сами тянем полосу к ближайшей
  // карточке пружинкой (CSS-снап ради этого выключен на всех брейкпоинтах).
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
    cancelAnimationFrame(frame.current);
    frame.current = requestAnimationFrame(sync);
    if (glideActive.current) {
      return;
    }
    if (settleTimer.current) {
      clearTimeout(settleTimer.current);
    }
    settleTimer.current = setTimeout(settle, SETTLE_IDLE_MS);
  }, [sync, settle]);

  const goTo = useCallback(
    (index: number) => {
      const el = scroller.current;
      const card = cards.current[index];
      if (!el || !card) {
        return;
      }
      glideTo(
        desk
          ? card.offsetLeft + card.offsetWidth / 2 - el.clientWidth / 2
          : card.offsetLeft - TOUCH_INSET,
      );
    },
    [desk, glideTo],
  );

  return (
    <div className="flex flex-col">
      <div className="order-2 mt-6 flex items-center justify-center gap-4 desk:mt-12">
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
      <div
        ref={scroller}
        onScroll={onScroll}
        onPointerDown={stopGlide}
        onWheel={stopGlide}
        className="relative order-1 flex gap-3 overflow-x-auto overscroll-x-contain px-6 pb-2 [scrollbar-width:none] desk:px-[calc((100%_-_320px)/2)] [&::-webkit-scrollbar]:hidden"
      >
        {REVIEWS.map((review, i) => (
          <figure
            key={review.name}
            ref={(el) => {
              cards.current[i] = el;
            }}
            className="relative h-[500px] w-[320px] shrink-0 overflow-clip rounded-[32px] desk:will-change-transform"
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
        <div aria-hidden className="w-[calc(100%_-_332px)] shrink-0 desk:hidden" />
      </div>
    </div>
  );
}
