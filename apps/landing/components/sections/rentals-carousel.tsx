"use client";

import Image, { type StaticImageData } from "next/image";
import { useCallback, useRef, useState } from "react";
import rentalCalendar from "@/assets/sections/rental-calendar.webp";
import rentalContacts from "@/assets/sections/rental-contacts.webp";
import rentalContract from "@/assets/sections/rental-contract.webp";
import rentalHistory from "@/assets/sections/rental-history.webp";
import rentalOverdue from "@/assets/sections/rental-overdue.webp";
import rentalReport from "@/assets/sections/rental-report.webp";

// Карусель «Управляйте арендой» — макет 2814-884: 6 карточек 380×550
// (radius-40, #f3f4f6, px-40 py-52), зазор 20, иллюстрации прижаты к низу
// по макету каждой карточки; 3 точки-страницы (12×12, шаг 16).
type Card = {
  title: string;
  text: string;
  textSmall: boolean;
  img: StaticImageData;
  alt: string;
  // Геометрия иллюстрации из макета: ширина и отступ от низа карточки.
  imgWidth: number;
  imgBottom: number;
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
  },
  {
    title: "Напомним, если платеж просрочится",
    text: "Если вовремя не отметить оплату, платеж станет просроченным",
    textSmall: false,
    img: rentalOverdue,
    alt: "Экран просроченного платежа в Рентли",
    imgWidth: 240,
    imgBottom: 70,
  },
  {
    title: "Отслеживайте сроки договора",
    text: "Напомним, когда договор будет подходить к концу",
    textSmall: true,
    img: rentalContract,
    alt: "Экран договора аренды в Рентли",
    imgWidth: 240,
    imgBottom: 70,
  },
  {
    title: "Получайте отчет об итогах аренды",
    text: "После завершения, покажем прибыль за период аренды",
    textSmall: false,
    img: rentalReport,
    alt: "Экран отчета по аренде в Рентли",
    imgWidth: 380,
    imgBottom: 0,
  },
  {
    title: "Добавляйте контакты арендатаров",
    text: "Контакты арендатаров в одном месте",
    textSmall: false,
    img: rentalContacts,
    alt: "Экран контактов арендаторов в Рентли",
    imgWidth: 320,
    imgBottom: 0,
  },
  {
    title: "Возвращайтесь к прошлым арендам",
    text: "История аренд сохраняется, к ней можно вернуться в любой момент",
    textSmall: false,
    img: rentalHistory,
    alt: "Экран истории аренд в Рентли",
    imgWidth: 380,
    imgBottom: 0,
  },
];

const DOTS = 3;

export function RentalsCarousel() {
  const scroller = useRef<HTMLDivElement>(null);
  const [active, setActive] = useState(0);

  const onScroll = useCallback(() => {
    const el = scroller.current;
    if (!el) {
      return;
    }
    const max = el.scrollWidth - el.clientWidth;
    setActive(Math.round((el.scrollLeft / max) * (DOTS - 1)) || 0);
  }, []);

  const goTo = useCallback((index: number) => {
    const el = scroller.current;
    if (!el) {
      return;
    }
    const max = el.scrollWidth - el.clientWidth;
    el.scrollTo({ left: (max * index) / (DOTS - 1), behavior: "smooth" });
  }, []);

  return (
    <>
      <div
        ref={scroller}
        onScroll={onScroll}
        className="flex snap-x snap-mandatory gap-5 overflow-x-auto px-[max(8px,calc((100vw-1000px)/2))] pb-2 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
      >
        {CARDS.map((card) => (
          <article
            key={card.title}
            className="relative h-[550px] w-[300px] shrink-0 snap-start overflow-clip rounded-[40px] bg-surface px-10 py-[52px] desk:w-[380px]"
          >
            <div className="flex flex-col items-center gap-3 text-center">
              <h3 className="text-m font-medium leading-6 desk:text-[28px] desk:leading-8">
                {card.title}
              </h3>
              <p
                className={`text-gray-2 ${card.textSmall ? "text-xs leading-[18px]" : "text-r"}`}
              >
                {card.text}
              </p>
            </div>
            <Image
              src={card.img}
              alt={card.alt}
              width={card.imgWidth}
              height={card.imgWidth}
              sizes="(min-width: 1200px) 380px, 300px"
              className="absolute bottom-0 left-1/2 h-auto -translate-x-1/2"
              style={{ maxWidth: card.imgWidth, marginBottom: card.imgBottom }}
            />
          </article>
        ))}
      </div>
      <div className="mt-2 flex items-center justify-center gap-4">
        {Array.from({ length: DOTS }, (_, i) => (
          <button
            key={i}
            type="button"
            aria-label={`Страница ${i + 1}`}
            aria-current={active === i}
            onClick={() => goTo(i)}
            className={`size-3 rounded-full transition-colors duration-200 ${
              active === i ? "bg-primary" : "bg-line hover:bg-gray-3"
            }`}
          />
        ))}
      </div>
    </>
  );
}
