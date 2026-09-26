"use client";

import Image, { type StaticImageData } from "next/image";
import { useCallback, useRef, useState } from "react";
import slide1 from "@/assets/sections/slide-1.webp";
import slide2 from "@/assets/sections/slide-2.webp";
import slide3 from "@/assets/sections/slide-3.webp";
import slide4 from "@/assets/sections/slide-4.webp";
import slide5 from "@/assets/sections/slide-5.webp";
import reviewAvatar1 from "@/assets/sections/review-avatar-1.webp";
import reviewAvatar2 from "@/assets/sections/review-avatar-2.webp";
import reviewAvatar3 from "@/assets/sections/review-avatar-3.webp";
import reviewAvatar4 from "@/assets/sections/review-avatar-4.webp";
import reviewAvatar5 from "@/assets/sections/review-avatar-5.webp";

// «Опыт пользователей», слайды — макет 2814-979 (десктоп): 5 фото
// (первое 338×450, остальные 300×400, radius-40, зазор 20), лента
// уезжает вправо за колонку; 5 точек 12×12 по центру: активная — чёрная.
const SLIDES: { src: StaticImageData; alt: string; width: number; height: number }[] = [
  { src: slide1, alt: "Квартира в приложении Рентли", width: 338, height: 450 },
  { src: slide2, alt: "Комната в приложении Рентли", width: 300, height: 400 },
  { src: slide3, alt: "Дом в приложении Рентли", width: 300, height: 400 },
  { src: slide4, alt: "Офис в приложении Рентли", width: 300, height: 400 },
  { src: slide5, alt: "Склад в приложении Рентли", width: 300, height: 400 },
];

export function TestimonialsSlides() {
  const scroller = useRef<HTMLDivElement>(null);
  const [active, setActive] = useState(0);

  const onScroll = useCallback(() => {
    const el = scroller.current;
    if (!el) {
      return;
    }
    const max = el.scrollWidth - el.clientWidth;
    if (max <= 0) {
      return;
    }
    setActive(Math.round((el.scrollLeft / max) * (SLIDES.length - 1)));
  }, []);

  const goTo = useCallback((index: number) => {
    const el = scroller.current;
    if (!el) {
      return;
    }
    const max = el.scrollWidth - el.clientWidth;
    el.scrollTo({ left: (max * index) / (SLIDES.length - 1), behavior: "smooth" });
  }, []);

  return (
    <div className="w-full">
      <div
        ref={scroller}
        onScroll={onScroll}
        className="flex snap-x snap-mandatory items-center gap-5 overflow-x-auto pb-2 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
      >
        {SLIDES.map((slide) => (
          <Image
            key={slide.alt}
            src={slide.src}
            alt={slide.alt}
            width={slide.width}
            height={slide.height}
            sizes="338px"
            style={{ width: slide.width, height: slide.height }}
            className="shrink-0 snap-start rounded-[40px] object-cover"
          />
        ))}
      </div>
      <div className="mt-12 flex items-center justify-center gap-4">
        {SLIDES.map((slide, index) => (
          <button
            key={slide.alt}
            type="button"
            aria-label={`Слайд ${index + 1}`}
            aria-current={active === index}
            onClick={() => goTo(index)}
            className={`size-3 rounded-full transition-colors duration-200 ${
              active === index ? "bg-ink" : "bg-line hover:bg-gray-3"
            }`}
          />
        ))}
      </div>
    </div>
  );
}

// Карусель отзывов для планшета и мобайла — макеты 2871:5925 / 2837:152224:
// 5 карточек 320×500 (radius-32, фото на фоне), тёмная стеклянная плашка
// (304×…, p-24, r24, rgba(0,0,0,.24), ring белый 32%, blur-16) прижата
// к низу: цитата 20/24 Medium + автор (аватар 44, имя 16/20, подпись
// 14/18 white/70); точки — как на десктопе.
const REVIEWS: {
  quote: string;
  name: string;
  sub: string;
  avatar: StaticImageData;
  photo: StaticImageData;
  alt: string;
}[] = [
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

export function TestimonialsReviews() {
  const scroller = useRef<HTMLDivElement>(null);
  const [active, setActive] = useState(0);

  const onScroll = useCallback(() => {
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

  const goTo = useCallback((index: number) => {
    const el = scroller.current;
    if (!el) {
      return;
    }
    const max = el.scrollWidth - el.clientWidth;
    el.scrollTo({ left: (max * index) / (REVIEWS.length - 1), behavior: "smooth" });
  }, []);

  return (
    <div className="w-full">
      <div
        ref={scroller}
        onScroll={onScroll}
        className="flex snap-x snap-mandatory gap-3 overflow-x-auto px-6 pb-2 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
      >
        {REVIEWS.map((review) => (
          <figure
            key={review.name}
            className="relative h-[500px] w-[320px] shrink-0 snap-start overflow-clip rounded-[32px]"
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
                  <p className="max-w-[180px] text-xs leading-[18px] text-white/70">{review.sub}</p>
                </div>
              </div>
            </figcaption>
          </figure>
        ))}
      </div>
      <div className="mt-6 flex items-center justify-center gap-4">
        {REVIEWS.map((review, index) => (
          <button
            key={review.name}
            type="button"
            aria-label={`Слайд ${index + 1}`}
            aria-current={active === index}
            onClick={() => goTo(index)}
            className={`size-3 rounded-full transition-colors duration-200 ${
              active === index ? "bg-ink" : "bg-line hover:bg-gray-3"
            }`}
          />
        ))}
      </div>
    </div>
  );
}
