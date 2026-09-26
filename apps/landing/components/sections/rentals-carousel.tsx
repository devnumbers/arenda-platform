"use client";

import Image, { type StaticImageData } from "next/image";
import { useCallback, useEffect, useRef, useState } from "react";
import rentalCalendar from "@/assets/sections/rental-calendar.webp";
import rentalContacts from "@/assets/sections/rental-contacts.webp";
import rentalContract from "@/assets/sections/rental-contract.webp";
import rentalHistory from "@/assets/sections/rental-history.webp";
import rentalOverdue from "@/assets/sections/rental-overdue.webp";
import rentalReport from "@/assets/sections/rental-report.webp";

// Карусель «Управляйте арендой» — макеты 2814-884 (десктоп: 6 карточек
// 380×550, r40, px-40 py-52, 3 точки) и 2859-3475 (планшет/мобайл:
// 320×500, r32, p-48/24, титул 22/26, текст 16/20, по точке на карточку —
// 6 точек; иллюстрации прижаты к низу по макету каждой карточки).
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
    title: "Добавляйте контакты арендатаров",
    text: "Контакты арендатаров в одном месте",
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

// Точки пагинации: на десктопе 3 (шаг по две карточки), на планшете и
// мобайле — по одной на карточку (6), макеты 2814-884 / 2859-3475.
const DESK_QUERY = "(min-width: 1200px)";

export function RentalsCarousel() {
  const scroller = useRef<HTMLDivElement>(null);
  const [desk, setDesk] = useState(false);
  const dots = desk ? 3 : 6;
  const [active, setActive] = useState(0);

  useEffect(() => {
    const mq = window.matchMedia(DESK_QUERY);
    const update = () => setDesk(mq.matches);
    update();
    mq.addEventListener("change", update);
    return () => mq.removeEventListener("change", update);
  }, []);

  const onScroll = useCallback(() => {
    const el = scroller.current;
    if (!el) {
      return;
    }
    const max = el.scrollWidth - el.clientWidth;
    setActive(Math.round((el.scrollLeft / max) * (dots - 1)) || 0);
  }, [dots]);

  const goTo = useCallback(
    (index: number) => {
      const el = scroller.current;
      if (!el) {
        return;
      }
      const max = el.scrollWidth - el.clientWidth;
      el.scrollTo({ left: (max * index) / (dots - 1), behavior: "smooth" });
    },
    [dots],
  );

  return (
    <>
      <div
        ref={scroller}
        onScroll={onScroll}
        className="flex snap-x snap-mandatory gap-3 overflow-x-auto px-6 pb-2 [scrollbar-width:none] desk:gap-5 desk:px-[max(20px,calc((100vw-1000px)/2))] [&::-webkit-scrollbar]:hidden"
      >
        {CARDS.map((card) => (
          <article
            key={card.title}
            className="relative h-[500px] w-[320px] shrink-0 snap-start overflow-clip rounded-[32px] bg-surface px-6 pt-12 desk:h-[550px] desk:w-[380px] desk:rounded-[40px] desk:px-10 desk:pt-[52px]"
          >
            <div className="mx-auto flex w-full max-w-[220px] flex-col items-center gap-2 text-center desk:gap-3">
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
      </div>
      <div className="mt-6 flex items-center justify-center gap-4 desk:mt-12">
        {Array.from({ length: dots }, (_, i) => (
          <button
            key={i}
            type="button"
            aria-label={`Страница ${i + 1}`}
            aria-current={active === i}
            onClick={() => goTo(i)}
            className={`size-3 rounded-full transition-colors duration-200 ${
              active === i ? "bg-ink" : "bg-line hover:bg-gray-3"
            }`}
          />
        ))}
      </div>
    </>
  );
}
