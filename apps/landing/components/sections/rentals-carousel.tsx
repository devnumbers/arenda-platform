"use client";

import Image, { type StaticImageData } from "next/image";
import { useCallback, useEffect, useRef, useState } from "react";
import rentalCalendar from "@/assets/sections/rental-calendar.webp";
import rentalContacts from "@/assets/sections/rental-contacts.webp";
import rentalContract from "@/assets/sections/rental-contract.webp";
import rentalHistory from "@/assets/sections/rental-history.webp";
import rentalOverdue from "@/assets/sections/rental-overdue.webp";
import rentalReport from "@/assets/sections/rental-report.webp";

// Карусель «Управляйте арендой» — макеты 2814-884/885/888 (десктоп) и
// 2859-3475 (планшет/мобайл): 6 карточек 380×550 (r40, px-40 py-52) на
// десктопе и 320×500 (r32, p-48/24) на планшете-мобайле; иллюстрации
// прижаты к низу по макету каждой карточки.
//
// Десктоп — «полка» в стиле Apple: активная карточка стоит по центру окна,
// по краям призрачное место (крайние карточки тоже встают по центру);
// клик по точке и докатка после свайпа — пружинка с лёгким перелётом цели
// и упругим возвратом (CSS-снап выключен везде, докатку ведём сами по
// простою скролла), соседние карточки чуть меньше и приглушены — по
// дистанции до центра в rAF, без внешних зависимостей. 6 точек-кнопок
// снизу по макету 2814-885 — на всех брейкпоинтах.
// Планшет/мобайл — та же логика (макет 2859-3475): карточки 320 с зазором
// 12, отступ 24px до первой и после последней (хвостовой спейсер даёт
// последней встать на ту же линию), тач-моментум нативный, докатка — наша
// пружинка; overscroll-x contain не даёт краевому свайпу дёргать навигацию.
// prefers-reduced-motion — без пружинки и «полки», скролл мгновенный.
type Card = {
  title: string;
  text: string;
  textSmall: boolean;
  img: StaticImageData;
  alt: string;
  // Геометрия иллюстрации из макета: ширина и отступ от низа карточки
  // (десктоп / планшет-мобайл).
  imgWidth: number;
  imgBottom: number;
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
    imgBottom: 70,
    imgBottomSub: 50,
  },
  {
    title: "Напомним, если платеж просрочится",
    text: "Если вовремя не отметить оплату, платеж станет просроченным",
    textSmall: false,
    img: rentalOverdue,
    alt: "Экран просроченного платежа в Рентли",
    imgWidth: 240,
    imgBottom: 70,
    imgBottomSub: 50,
  },
  {
    title: "Отслеживайте сроки договора",
    text: "Напомним, когда договор будет подходить к концу",
    textSmall: true,
    img: rentalContract,
    alt: "Экран договора аренды в Рентли",
    imgWidth: 240,
    imgBottom: 70,
    imgBottomSub: 50,
  },
  {
    title: "Получайте отчет об итогах аренды",
    text: "После завершения, покажем прибыль за период аренды",
    textSmall: false,
    img: rentalReport,
    alt: "Экран отчета по аренде в Рентли",
    imgWidth: 380,
    imgBottom: 0,
    imgBottomSub: 0,
  },
  {
    title: "Добавляйте контакты арендаторов",
    text: "Контакты арендаторов в одном месте",
    textSmall: false,
    img: rentalContacts,
    alt: "Экран контактов арендаторов в Рентли",
    imgWidth: 320,
    imgBottom: 0,
    imgBottomSub: 0,
  },
  {
    title: "Возвращайтесь к прошлым арендам",
    text: "История аренд сохраняется, к ней можно вернуться в любой момент",
    textSmall: false,
    img: rentalHistory,
    alt: "Экран истории аренд в Рентли",
    imgWidth: 380,
    imgBottom: 0,
    imgBottomSub: 0,
  },
];

const DESK_QUERY = "(min-width: 1200px)";

// Отступ первой/последней карточки на планшете-мобайле (макет 2859-3475):
// линия покоя карточки = её offsetLeft минус этот паддинг скроллера.
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

export function RentalsCarousel() {
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
        {CARDS.map((card, i) => (
          <button
            key={card.title}
            type="button"
            aria-label={card.title}
            aria-current={active === i}
            onClick={() => goTo(i)}
            className={`size-3 cursor-pointer rounded-full transition-colors duration-200 ${
              active === i ? "bg-ink" : "bg-line hover:bg-gray-3"
            }`}
          />
        ))}
      </div>
      <div
        ref={scroller}
        onScroll={onScroll}
        onPointerDown={stopGlide}
        onWheel={stopGlide}
        className="relative order-1 flex gap-3 overflow-x-auto overscroll-x-contain px-6 pb-2 [scrollbar-width:none] desk:gap-5 desk:px-[calc((100%_-_380px)/2)] [&::-webkit-scrollbar]:hidden"
      >
        {CARDS.map((card, i) => (
          <article
            key={card.title}
            ref={(el) => {
              cards.current[i] = el;
            }}
            className="relative h-[500px] w-[320px] shrink-0 overflow-clip rounded-[32px] bg-surface px-6 pt-12 desk:h-[550px] desk:w-[380px] desk:rounded-[40px] desk:px-10 desk:pt-[52px] desk:will-change-transform"
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
              className={`absolute bottom-0 left-1/2 h-auto -translate-x-1/2 ${
                card.imgBottomSub ? "mb-[50px] desk:mb-[70px]" : "mb-0"
              }`}
              style={{ maxWidth: card.imgWidth }}
            />
          </article>
        ))}
        {/* Хвостовой отступ 24px: последняя карточка встаёт на ту же линию,
            что и первая (100% здесь — контент-бокс без паддингов; 12px
            съедает flex-gap перед спейсером). */}
        <div
          aria-hidden
          className="w-[calc(100%_-_332px)] shrink-0 desk:hidden"
        />
      </div>
    </div>
  );
}
