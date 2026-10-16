import { Reveal } from "@/components/reveal";
import {
  TestimonialsCarousel,
  TestimonialsDesktop,
} from "@/components/sections/testimonials-carousel";

// «Опыт пользователей» — макеты 2814-979/980 (десктоп: центрированный H2,
// слева отзыв активной карточки 472px, справа лента фото, уезжающая вправо
// за колонку; шаг ряда 56) и 2837-152228 (планшет/мобайл: карусель
// отзыв-карточек во всю ширину, отступ 24px, шаг 32).
export function Testimonials() {
  return (
    <section id="testimonials" className="mt-24 scroll-mt-[88px] desk:mt-[156px] desk:scroll-mt-[104px]">
      <Reveal className="mx-auto w-full max-w-[1048px] px-6 desk:max-w-[1000px] desk:px-0">
        <h2 className="text-center text-[28px] font-semibold leading-8 tab:text-[32px] tab:leading-9 desk:text-h2 desk:leading-[60px]">
          Опыт пользователей
        </h2>
      </Reveal>
      {/* Оба варианта — карусель и десктоп — смонтированы всегда и прячутся
          CSS-ом (desk:hidden / hidden desk:block) — осознанно: мгновенное
          переключение брейкпоинта 1200px без потери скролл-позиции;
          matchMedia-размонтирование в духе rentals-carousel.tsx:152-155
          отвергнуто — сбросило бы скролл при пересечении брейкпоинта; цена —
          холостые matchMedia-листенеры и applyProgress в display:none
          (testimonials-carousel.tsx:417-438, :548-550), принята. */}
      <Reveal delay={100} className="mt-8 desk:hidden">
        <TestimonialsCarousel />
      </Reveal>
      <div className="mx-auto mt-14 hidden w-full max-w-[1048px] desk:block desk:max-w-[1000px] desk:px-0">
        <Reveal delay={100}>
          <TestimonialsDesktop />
        </Reveal>
      </div>
    </section>
  );
}
