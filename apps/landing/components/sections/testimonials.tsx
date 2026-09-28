import { Reveal } from "@/components/reveal";
import { TestimonialsCarousel } from "@/components/sections/testimonials-carousel";

// «Опыт пользователей» — макеты 2814-979 (десктоп: центрированный H2) и
// 2837-152228 (стрип отзыв-карточек: отзыв лежит прямо на фото, левой
// колонки с цитатой больше нет); карусель во всю ширину окна на всех
// брейкпоинтах, механика — канон «Управляйте арендой» (полка/пружинка).
export function Testimonials() {
  return (
    <section id="testimonials" className="mt-24 desk:mt-[156px]">
      <Reveal className="mx-auto mb-8 w-full max-w-[1048px] px-6 desk:mb-14 desk:max-w-[1000px] desk:px-0">
        <h2 className="text-center text-[28px] font-semibold leading-8 desk:text-h2 desk:leading-[60px]">
          Опыт пользователей
        </h2>
      </Reveal>
      <Reveal delay={100}>
        <TestimonialsCarousel />
      </Reveal>
    </section>
  );
}
