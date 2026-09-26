import Image from "next/image";
import { LandingLink } from "@/components/button";
import { Reveal } from "@/components/reveal";
import heroBg from "@/assets/sections/hero-bg.webp";
import heroBgTablet from "@/assets/sections/hero-bg-tablet.webp";
import heroBgMobile from "@/assets/sections/hero-bg-mobile.webp";

// Хиро — макеты 2814-731 (десктоп: панель 607, r40, контент внизу слева,
// H1 72/76), 2859-3457 (планшет: 936, r32, контент сверху по центру,
// H1 36/40 Medium), 2826-151757 (мобайл: 788, r32, H1 28/32 SemiBold,
// подзаголовок 16/20). У планшета и мобайла свои кропы фонового фото —
// imageRef-заливки узлов Figma, поэтому <picture> с media-источниками.
export function Hero() {
  return (
    <section id="hero" className="px-2 pt-2">
      <div className="relative flex h-[788px] flex-col justify-start overflow-hidden rounded-[32px] p-8 pt-12 tab:h-[936px] tab:px-12 tab:pt-16 tab:pb-12 desk:h-[607px] desk:justify-end desk:rounded-[40px] desk:p-10">
        <picture>
          <source media="(max-width: 480px)" srcSet={heroBgMobile.src} />
          <source media="(min-width: 481px) and (max-width: 1199px)" srcSet={heroBgTablet.src} />
          <Image
            src={heroBg}
            alt="Интерфейс Рентли на экране телефона на фоне квартиры"
            fill
            priority
            sizes="100vw"
            className="object-cover"
          />
        </picture>
        <Reveal>
          <div className="relative flex max-w-[855px] flex-col items-center gap-6 text-center desk:gap-8 desk:items-start desk:text-left">
            <div className="flex flex-col gap-4 [text-shadow:0_8px_24px_rgba(0,0,0,0.12)] desk:gap-6">
              <h1 className="text-balance text-[28px] font-semibold leading-8 text-white tab:text-[36px] tab:font-medium tab:leading-10 desk:text-h1 desk:font-semibold desk:leading-[76px]">
                Сервис управления арендой недвижимости
              </h1>
              <p className="text-balance text-s font-normal leading-5 text-white/70 tab:text-m tab:font-medium tab:leading-6 desk:text-h4 desk:font-medium desk:leading-8 desk:text-white">
                Управляйте сдачей жилья без таблиц и заметок
              </p>
            </div>
            <LandingLink variant="white" href="/login">
              Войти в сервис
            </LandingLink>
          </div>
        </Reveal>
      </div>
    </section>
  );
}
