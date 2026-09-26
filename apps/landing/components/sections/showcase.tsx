import Image from "next/image";
import { LandingLink } from "@/components/button";
import { Reveal } from "@/components/reveal";
import cardBar from "@/assets/sections/showcase-card-bar.webp";
import screens from "@/assets/sections/showcase-screens.webp";

// «Стройте арендный бизнес» — макеты 2814-738 (десктоп), 2859-3464
// (планшет: поля 40, полоса 1025×102.5, скриншоты 1106×484 cover),
// 2826-151232 (мобайл: полоса 650×65, скриншоты 503×220 cover): заголовок
// H4 28/32 + подпись 16/20 gray-2 (десктоп — 56 + 28/32), шаг 32; оба
// стрипа — вылезающие за колонку, клип на обёртке и overflow-x: clip.
export function Showcase() {
  return (
    <section id="showcase" className="mt-20 desk:mt-[156px]">
      <div className="mx-auto flex max-w-[1048px] flex-col items-center gap-8 px-10 desk:max-w-[1000px] desk:gap-14 desk:px-0">
        <Reveal className="w-full">
          <div className="flex w-full flex-col items-center gap-3 text-center desk:gap-6">
            <h2 className="text-balance text-[28px] font-semibold leading-8 desk:text-h2 desk:leading-[60px]">
              Создайте карточку своей недвижимости
            </h2>
            <p className="text-s leading-5 text-gray-2 desk:text-[28px] desk:leading-8">
              Добавьте квартиру, дом, помещение
            </p>
          </div>
        </Reveal>
        <Reveal delay={100} className="w-full">
          <div className="overflow-hidden">
            <Image
              src={cardBar}
              alt="Карточка объекта в Рентли"
              sizes="(min-width: 1200px) 1000px, 1025px"
              className="relative left-1/2 h-auto w-[650px] max-w-none -translate-x-1/2 tab:w-[1025px] desk:left-0 desk:w-full desk:translate-x-0"
            />
          </div>
        </Reveal>
        <Reveal delay={150} className="w-full">
          <div className="overflow-hidden">
            <Image
              src={screens}
              alt="Скриншоты экранов приложения Рентли"
              sizes="(min-width: 1200px) 1600px, 1106px"
              className="relative left-1/2 h-[220px] w-[503px] max-w-none -translate-x-1/2 object-cover tab:h-[484px] tab:w-[1106px] desk:left-0 desk:h-auto desk:w-[1600px] desk:translate-x-0"
            />
          </div>
        </Reveal>
        <Reveal delay={200}>
          <LandingLink href="/login">Попробовать</LandingLink>
        </Reveal>
      </div>
    </section>
  );
}
