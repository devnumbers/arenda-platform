"use client";

import Image, { type StaticImageData } from "next/image";
import { useEffect, useRef } from "react";
import Swiper from "swiper";
import { A11y, Mousewheel } from "swiper/modules";
import "swiper/css";
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
// Механика — полная копия эталона pay.yandex.ru/business/checkout,
// блок «Чекаут Яндекс Пэй открывает возможности экосистемы Яндекса»
// (живой дамп конфига и поведения Swiper 26.10): один инстанс,
// slidesPerView auto; петли нет — за краем сопротивление ядра
// (resistanceRatio 0.85) и возврат; drag 1:1 за мышью и пальцем
// (simulateTouch как в эталоне, grabCursor выключен — курсора «руки»
// нет), threshold 5, touchRatio 1, отпуск — снап к ближайшей позиции;
// клик по карточке ничего не делает (preventClicks /
// slideToClickedSlide: false — дефолты ядра, совпадают с эталоном).
// Скорости эталона: 360мс при окне ≥720 и 460мс при <720, кривая —
// дефолт ядра CSS ease; граница и уровни breakpoints живут на ширине
// ОКНА (breakpointsBase 'window' — дефолт ядра и эталона, канон ярусов
// DESIGN.md §2). Окно <720 — активная карточка по центру с
// подглядывающими краями соседей (centeredSlides + centeredSlidesBounds:
// первая карточка прижата к левому краю, последняя — к правому);
// ≥720 — карточка у левого края колонки, как сейчас. Shift+колесо и
// горизонтальный трекпад листают ленту — модуль Mousewheel forceToAxis
// (eventsTarget — контейнер, как в эталоне), вертикальное колесо
// скроллит страницу; вертикальный пан на таче — страница
// (touch-action: pan-y в core-CSS .swiper-horizontal). Автоплея нет —
// в конфиге эталона autoplay {delay: 3000} прописан, но фактически не
// стартует (autoplayRunning: false, живой замер 26.10); решение
// владельца 26.10. Буллетов пагинации нет — решение владельца 26.10
// (макеты 2967-75818 точек не имеют), клавиатурных стрелок нет.
// Метки слайдов «N / M» — модуль A11y с дефолтами ядра, как в эталоне.
//
// Уровни breakpoints ядра не каскадят: выбирается один — старший
// совпавший, и он extend'ится поверх params (исходники setBreakpoint),
// поэтому каждый уровень повторяет полный набор параметров. Смена
// уровня на ресайзе пересчитывает снапы сама (resize-хендлер ядра);
// геометрия карточек живёт в css-модуле на тех же границах окна.
//
// strip-engine из секции снят (сам движок остаётся в testimonials и
// tablet-strip).
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
    textSmall: false,
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
    textSmall: false,
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
  const scrollerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = scrollerRef.current;
    if (!el) {
      return;
    }
    const sw = new Swiper(el, {
      // a11y — метки слайдов «N / M» как в эталоне (slideLabelMessage —
      // дефолт модуля); в core-сборке модуль не включён по умолчанию
      modules: [A11y, Mousewheel],
      slidesPerView: "auto",
      // мышиный драг на десктопе — как в эталоне; дефолт ядра true,
      // ставим явно против смены дефолтов
      simulateTouch: true,
      mousewheel: {
        enabled: true,
        forceToAxis: true,
      },
      // База — ярус планшета-мобайла; уровни ниже повторяют полный
      // набор (см. комментарий о не-каскаде уровней выше)
      speed: 460,
      centeredSlides: true,
      centeredSlidesBounds: true,
      spaceBetween: 12,
      breakpoints: {
        0: { speed: 460, centeredSlides: true, centeredSlidesBounds: true, spaceBetween: 12 },
        720: { speed: 360, centeredSlides: false, centeredSlidesBounds: false, spaceBetween: 12 },
        1200: { speed: 360, centeredSlides: false, centeredSlidesBounds: false, spaceBetween: 20 },
      },
    });
    return () => {
      sw.destroy(true, true);
    };
  }, []);

  return (
    <div className={s.viewport}>
      <div ref={scrollerRef} className={`swiper swiper-horizontal select-none ${s.scroller}`}>
        <div className="swiper-wrapper">
          {CARDS.map((card) => (
            <article
              key={card.title}
              className={`swiper-slide ${s.slide} relative overflow-clip rounded-[32px] bg-surface px-6 pt-12 desk:rounded-[40px] desk:px-10 desk:pt-[52px]`}
            >
              {/* Колонка текста — макет: 272 (карточка 320 минус паддинги
                  24×2) на планшете-мобиле, 300 на десктопе (380−2×40). При
                  прежних 220 заголовки «Напомним…» и «Добавляйте контакты…»
                  ломались на 3 строки вместо двух (#1517). */}
              <div className="mx-auto flex w-full max-w-[272px] flex-col items-center gap-2 text-center desk:gap-3 desk:max-w-[300px]">
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
                {/* max-width не выше карточки: в макете мобилы фото
                    последних трёх карточек — ровно во всю ширину 320
                    (верх y=180), а не 380 с обрезом краями карточки,
                    поднимавший рисунок на текст (#1517). */}
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
                  style={{ maxWidth: `min(100%, ${card.imgWidth}px)` }}
                />
            </article>
          ))}
        </div>
      </div>
    </div>
  );
}
