"use client";

import Image, { type StaticImageData } from "next/image";
import { useEffect, useRef } from "react";
import { createStripEngine } from "@/components/strip-engine";
import rentalCalendar from "@/assets/sections/rental-calendar.webp";
import rentalContacts from "@/assets/sections/rental-contacts.webp";
import rentalContract from "@/assets/sections/rental-contract.webp";
import rentalHistory from "@/assets/sections/rental-history.webp";
import rentalOverdue from "@/assets/sections/rental-overdue.webp";
import rentalReport from "@/assets/sections/rental-report.webp";
import s from "./rentals-carousel.module.css";

// Карусель «Управляйте арендой» — макеты 2967-75818 (ПК), 3005-78042
// (планшет), 3008-79124 (мобила): 6 карточек 380×550 (r40) на десктопе
// и 320×500 (r32) на планшете-мобиле; иллюстрации прижаты к низу по
// макету каждой карточки. Лента стартует от левого края контентной
// колонки (на ПК 1000px, на планшете-мобиле поле 24px) и течёт за её
// правый край до обреза вьюпортом. Точек и «полки» нет — карточки
// равные, без масштаба и приглушения.
//
// Движок — общий модуль лент лендинга (components/strip-engine.ts),
// канон 1:1 с каруселью pay.yandex.ru/business/acquiring (замер 02.10):
// drag 1:1 за указателем без инерции со снапом и броском, Shift+Scroll —
// шаг на событие, резиновый край, 360мс ease-out (460мс <720). Ярусы
// геометрии — зеркало CSS-модуля: ПК — колонка 1000px, планшет-мобайл —
// колонка «ширина окна − 48» (поле 24px). Ширины карточек движку не
// нужны — шаг и длина стрипа измеряются по живым детям дорожки.
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

export function RentalsCarousel() {
  const viewportRef = useRef<HTMLDivElement>(null);
  const trackRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const vp = viewportRef.current;
    const track = trackRef.current;
    if (!vp || !track) {
      return;
    }
    const engine = createStripEngine(vp, track, {
      tier: () =>
        window.matchMedia("(min-width: 1200px)").matches
          ? { gap: 20, column: () => 1000 }
          : { gap: 12, column: (w) => w - 48 },
      draggingClass: s.dragging,
    });
    return () => {
      engine.destroy();
    };
  }, []);

  return (
    <div ref={viewportRef} className={s.viewport}>
      <div ref={trackRef} className={s.track}>
        {CARDS.map((card) => (
          <article
            key={card.title}
            className="relative h-[500px] w-[320px] shrink-0 overflow-clip rounded-[32px] bg-surface px-6 pt-12 desk:h-[550px] desk:w-[380px] desk:rounded-[40px] desk:px-10 desk:pt-[52px]"
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
              draggable={false}
              className={`absolute bottom-0 left-1/2 h-auto -translate-x-1/2 ${
                card.imgBottomSub ? "mb-[50px] desk:mb-[70px]" : "mb-0"
              }`}
              style={{ maxWidth: card.imgWidth }}
            />
          </article>
        ))}
      </div>
    </div>
  );
}
