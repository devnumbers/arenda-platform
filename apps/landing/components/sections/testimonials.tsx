import Image from "next/image";
import { Reveal } from "@/components/reveal";
import { TestimonialsReviews, TestimonialsSlides } from "@/components/sections/testimonials-slides";
import avatar from "@/assets/sections/testimonial-avatar.webp";

// «Опыт пользователей» — макеты 2814-979 (десктоп: H2 56, колонка цитаты
// 36/40 + автор под ней (аватар 64), лента слайдов, шаг 76) и 2871:5925 /
// 2837:152224 (планшет/мобайл: карусель отзыв-карточек с цитатами).
export function Testimonials() {
  return (
    <section id="testimonials" className="mt-24 desk:mt-[156px]">
      <div className="mx-auto w-full max-w-[1048px] px-6 desk:max-w-[1000px] desk:px-0">
        <Reveal>
          <h2 className="text-center text-[28px] font-semibold leading-8 desk:text-h2 desk:leading-[60px]">
            Опыт пользователей
          </h2>
        </Reveal>
        <Reveal delay={100}>
          {/* Планшет/мобайл — карусель отзывов; десктоп — цитата + лента. */}
          <div className="mt-8 desk:hidden">
            <TestimonialsReviews />
          </div>
          <div className="mt-14 hidden flex-col gap-[76px] desk:flex desk:flex-row">
            <div className="flex w-[452px] shrink-0 flex-col gap-8">
              <blockquote className="text-h3 font-medium leading-10">
                Приложение заменило мне заметки и Excel
              </blockquote>
              <div className="flex items-center gap-4">
                <Image
                  src={avatar}
                  alt="Дмитрий"
                  width={64}
                  height={64}
                  className="size-16 rounded-[100px] object-cover"
                />
                <div className="flex flex-col gap-2">
                  <p className="text-m font-medium leading-6">Дмитрий</p>
                  <p className="text-r text-gray-2">
                    Сдает квартиру уже второй год
                  </p>
                </div>
              </div>
            </div>
            <TestimonialsSlides />
          </div>
        </Reveal>
      </div>
    </section>
  );
}
