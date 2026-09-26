import Image from "next/image";
import { LandingLink } from "@/components/button";
import { Reveal } from "@/components/reveal";
import heroBg from "@/assets/sections/hero-bg.webp";

// Хиро — макет 2814-731: панель radius-40 (мобайл 32) с фоновым фото,
// контент прижат вниз-влево (десктоп) / по центру (мобайл): H1 72/76 белым
// с тенью 0 8 24 rgba(0,0,0,.12), подзаголовок 28/32 Medium, белая кнопка.
export function Hero() {
  return (
    <section id="hero" className="px-2">
      {/* Мобильный макет: контент сверху по центру (pt-48); десктоп — внизу слева. */}
      <div className="relative flex min-h-[560px] flex-col justify-start overflow-hidden rounded-[32px] p-8 pt-12 desk:h-[607px] desk:min-h-0 desk:justify-end desk:rounded-[40px] desk:p-10">
        <Image
          src={heroBg}
          alt="Интерфейс Рентли на экране телефона на фоне квартиры"
          fill
          priority
          sizes="100vw"
          className="object-cover"
        />
        <Reveal>
          <div className="relative flex max-w-[855px] flex-col items-center gap-8 text-center desk:items-start desk:text-left">
            <div className="flex flex-col gap-6 [text-shadow:0_8px_24px_rgba(0,0,0,0.12)]">
              <h1 className="text-[28px] font-semibold leading-8 text-white desk:text-h1 desk:leading-[76px]">
                Сервис управления арендой недвижимости
              </h1>
              <p className="text-m font-medium leading-6 text-white/70 desk:text-h4 desk:font-medium desk:leading-8 desk:text-white">
                Управляйте сдачей жилья без таблиц и заметок
              </p>
            </div>
            <LandingLink variant="white" href="/login" className="min-h-14 desk:min-h-16">
              Войти в сервис
            </LandingLink>
          </div>
        </Reveal>
      </div>
    </section>
  );
}
