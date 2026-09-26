import Image from "next/image";
import { LandingLink } from "@/components/button";
import { Reveal } from "@/components/reveal";
import cardBar from "@/assets/sections/showcase-card-bar.webp";
import screens from "@/assets/sections/showcase-screens.webp";

// «Стройте арендный бизнес» — макет 2814-738: центрированный заголовочный
// блок (H2 56 + подпись 28 gray-2, шаг 24), полоса-фрагмент 3000×300,
// широкий скриншот 1600×400 с вылезанием за колонку, синяя кнопка; шаг 56.
export function Showcase() {
  return (
    <section id="showcase" className="mt-20 desk:mt-[156px]">
      <div className="mx-auto flex max-w-[1000px] flex-col items-center gap-12 px-5 desk:gap-14 desk:px-10">
        <Reveal className="w-full">
          <div className="flex w-full flex-col items-center gap-6 text-center">
            <h2 className="text-[28px] font-semibold leading-8 desk:text-h2 desk:leading-[60px]">
              Создайте карточку своей недвижимости
            </h2>
            <p className="text-r desk:text-[28px] desk:leading-8 desk:text-gray-2">
              Добавьте квартиру, дом, помещение
            </p>
          </div>
        </Reveal>
        <Reveal delay={100} className="w-full">
          <Image
            src={cardBar}
            alt="Карточка объекта в Рентли"
            sizes="(min-width: 1200px) 1000px, 100vw"
            className="h-auto w-full"
          />
        </Reveal>
        <Reveal delay={150} className="w-full">
          {/* Стрип шире колонки: центрированный вылезающий блок 1600×400;
              клип на обёртке — иначе 200vw расширяет канву страницы. */}
          <div className="overflow-hidden">
            <div className="relative left-1/2 w-[200vw] -translate-x-1/2 desk:w-[1600px]">
              <Image
                src={screens}
                alt="Скриншоты экранов приложения Рентли"
                sizes="1600px"
                className="h-auto w-full"
              />
            </div>
          </div>
        </Reveal>
        <Reveal delay={200}>
          <LandingLink href="/login">Попробовать</LandingLink>
        </Reveal>
      </div>
    </section>
  );
}
