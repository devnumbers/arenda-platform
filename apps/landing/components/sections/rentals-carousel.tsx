"use client";

import Image, { type StaticImageData } from "next/image";
import { useCallback, useEffect, useRef, useState } from "react";
import { CarouselDots } from "@/components/carousel-dots";
import {
  createSpring,
  nearestCardIndex,
  type Spring,
} from "@/components/carousel-spring";
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
// и упругим возвратом (общая для лент — components/carousel-spring.ts;
// CSS-снап выключен везде, докатку ведём сами по простою скролла),
// соседние карточки чуть меньше и приглушены — по
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
  // Геометрия иллюстрации из макета: ширина; отступ иллюстрации от низа
  // карточки задаёт флаг imgBottomSub — 50px по макету (в разметке это
  // mb-[50px] на планшете-мобайле и desk:mb-[70px] на десктопе), 0 —
  // иллюстрация прижата к низу.
  imgWidth: number;
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
    imgBottomSub: 50,
  },
  {
    title: "Напомним, если платеж просрочится",
    text: "Если вовремя не отметить оплату, платеж станет просроченным",
    textSmall: false,
    img: rentalOverdue,
    alt: "Экран просроченного платежа в Рентли",
    imgWidth: 240,
    imgBottomSub: 50,
  },
  {
    title: "Отслеживайте сроки договора",
    text: "Напомним, когда договор будет подходить к концу",
    textSmall: true,
    img: rentalContract,
    alt: "Экран договора аренды в Рентли",
    imgWidth: 240,
    imgBottomSub: 50,
  },
  {
    title: "Получайте отчет об итогах аренды",
    text: "После завершения, покажем прибыль за период аренды",
    textSmall: false,
    img: rentalReport,
    alt: "Экран отчета по аренде в Рентли",
    imgWidth: 380,
    imgBottomSub: 0,
  },
  {
    title: "Добавляйте контакты арендаторов",
    text: "Контакты арендаторов в одном месте",
    textSmall: false,
    img: rentalContacts,
    alt: "Экран контактов арендаторов в Рентли",
    imgWidth: 320,
    imgBottomSub: 0,
  },
  {
    title: "Возвращайтесь к прошлым арендам",
    text: "История аренд сохраняется, к ней можно вернуться в любой момент",
    textSmall: false,
    img: rentalHistory,
    alt: "Экран истории аренд в Рентли",
    imgWidth: 380,
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

export function RentalsCarousel() {
  // relative на скроллере делает offsetLeft карточек координатами скролла —
  // на них построены и глайд, и «полка».
  const scroller = useRef<HTMLDivElement>(null);
  const cards = useRef<Array<HTMLElement | null>>([]);
  const frame = useRef(0);
  const reduced = useRef(false);
  const [desk, setDesk] = useState(false);
  const [active, setActive] = useState(0);
  const activeRef = useRef(0);
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
    // Активная карточка — ближайшая к целевой точке; поиск общий с докаткой
    // (nearestCardIndex), поверх здесь идёт отрисовка «полки» по дистанции.
    const best = nearestCardIndex(nodes, target);
    if (best < 0) {
      return;
    }
    for (let i = 0; i < nodes.length; i += 1) {
      const card = nodes[i];
      if (!card) {
        continue;
      }
      const dist = Math.abs(card.offsetLeft + card.offsetWidth / 2 - target);
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
    },
    [],
  );

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
    const card = nodes[nearestCardIndex(nodes, target)];
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
    cancelAnimationFrame(frame.current);
    frame.current = requestAnimationFrame(sync);
    springRef.current?.scheduleSettle(settle);
  }, [sync, settle]);

  const goTo = useCallback(
    (index: number) => {
      const el = scroller.current;
      const card = cards.current[index];
      if (!el || !card) {
        return;
      }
      springRef.current?.glideTo(
        desk
          ? card.offsetLeft + card.offsetWidth / 2 - el.clientWidth / 2
          : card.offsetLeft - TOUCH_INSET,
      );
    },
    [desk],
  );

  return (
    <div className="flex flex-col">
      <div className="order-2 mt-6 flex items-center justify-center gap-4 desk:mt-12">
        <CarouselDots
          count={CARDS.length}
          active={active}
          onSelect={goTo}
          labelFor={(i) => CARDS[i]?.title ?? ""}
        />
      </div>
      <div
        ref={scroller}
        onScroll={onScroll}
        onPointerDown={() => springRef.current?.stopGlide()}
        onWheel={() => springRef.current?.stopGlide()}
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
