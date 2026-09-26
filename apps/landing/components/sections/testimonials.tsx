import Image from "next/image";
import { Reveal } from "@/components/reveal";
import { TestimonialsSlides } from "@/components/sections/testimonials-slides";
import avatar from "@/assets/sections/testimonial-avatar.webp";

// «Опыт пользователей» — макет 2814-979: H2 56 по центру; строка из двух
// половин (шаг 56): цитата 36/40 Medium + автор (аватар 64, имя 20/24
// Medium, подпись 18/22 gray-2, шаги 32/16/8) и лента фото-слайдов.
export function Testimonials() {
  return (
    <section id="testimonials" className="mt-20 desk:mt-[156px]">
      <div className="mx-auto w-full max-w-[1000px] px-5 desk:px-10">
        <Reveal>
          <h2 className="text-center text-[28px] font-semibold leading-8 desk:text-h2 desk:leading-[60px]">
            Опыт пользователей
          </h2>
        </Reveal>
        <Reveal delay={100}>
          <div className="mt-14 flex flex-col gap-10 desk:flex-row desk:gap-14">
            <div className="flex shrink-0 flex-col justify-between gap-8 desk:w-[452px] desk:self-stretch">
              <blockquote className="text-xl font-medium leading-6 desk:text-h3 desk:leading-10">
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
