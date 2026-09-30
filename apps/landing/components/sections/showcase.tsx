import Image, { getImageProps } from "next/image";
import { LandingLink } from "@/components/button";
import { Reveal } from "@/components/reveal";
import cardBar from "@/assets/sections/showcase-card-bar.webp";
import screens from "@/assets/sections/showcase-screens.webp";
import screensSub from "@/assets/sections/showcase-screens-sub.webp";

// «Стройте арендный бизнес» — макеты 2814-738 (десктоп), 2859-3464
// (планшет: поля 40, полоса 1025×102.5, скриншоты 1106×484), 2826-151232
// (мобайл: полоса 650×65, скриншоты 503×220): заголовок H4 28/32 +
// подпись 16/20 gray-2 (десктоп — 56 + 28/32), шаг 32. Оба стрипа —
// вылезающие за колонку и отцентрованные по странице; клипает их общий
// overflow-x: clip на html/body, у планшета/мобайла свой кроп скриншотов
// (4096×1792 из Figma, соотношение 1106/484), у десктопа — 1600×400.
export function Showcase() {
  // Ветка ≤1199 считается через getImageProps, чтобы планшетный кроп
  // (4096×1792) шёл через /_next/image-оптимизатор (AVIF q90), а не сырым
  // webp; слоты 503/1106 — это w-[503px] и tab:w-[1106px] у Image ниже.
  const screensSubImg = getImageProps({
    src: screensSub,
    alt: "",
    sizes: "(max-width: 480px) 503px, 1106px",
    quality: 90,
  });
  return (
    <section id="showcase" className="mt-24 scroll-mt-[88px] desk:mt-[156px] desk:scroll-mt-[104px]">
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
          <Image
            src={cardBar}
            alt="Карточка объекта в Рентли"
            sizes="(min-width: 1200px) 1000px, (min-width: 481px) 1025px, 650px"
            className="relative left-1/2 h-auto w-[650px] max-w-none -translate-x-1/2 tab:w-[1025px] desk:w-full"
          />
        </Reveal>
        <Reveal delay={150} className="w-full">
          <picture>
            <source
              media="(max-width: 1199px)"
              srcSet={screensSubImg.props.srcSet}
              sizes={screensSubImg.props.sizes}
            />
            <Image
              src={screens}
              alt="Скриншоты экранов приложения Рентли"
              sizes="(min-width: 1200px) 1600px, 1106px"
              className="relative left-1/2 h-auto w-[503px] max-w-none -translate-x-1/2 tab:w-[1106px] desk:w-[1600px]"
            />
          </picture>
        </Reveal>
        <Reveal delay={200}>
          <LandingLink href="/login">Попробовать</LandingLink>
        </Reveal>
      </div>
    </section>
  );
}
