"use client";

import Image, { type StaticImageData } from "next/image";
import { useCallback, useRef, useState } from "react";
import slide1 from "@/assets/sections/slide-1.webp";
import slide2 from "@/assets/sections/slide-2.webp";
import slide3 from "@/assets/sections/slide-3.webp";
import slide4 from "@/assets/sections/slide-4.webp";
import slide5 from "@/assets/sections/slide-5.webp";

// Слайды «Опыта пользователей» — макет 2814-979: 5 фото (первое 338×450,
// остальные 300×400, radius-40, зазор 20), лента уезжает вправо за колонку;
// 5 точек 12×12: активная — чёрная (#171a1c), остальные #ebebeb.
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
            sizes="(min-width: 1200px) 338px, 70vw"
            style={{ width: slide.width, height: slide.height }}
            className="shrink-0 snap-start rounded-[40px] object-cover"
          />
        ))}
      </div>
      <div className="mt-6 flex items-center justify-center gap-4 desk:justify-start">
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
