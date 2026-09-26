import Image from "next/image";
import { LandingLink } from "@/components/button";
import { Reveal } from "@/components/reveal";
import ctaLogo from "@/assets/sections/cta-logo-3d.webp";

// Финальный CTA — макет 2814-1142: синяя панель-градиент #88b7ff→#2b7fff
// (radius-40, py-156), логотип-3D 156×156, H2 56 белым, подпись 22/26
// white/80 (шаг 16), белая кнопка «Начать бесплатно».
export function Cta() {
  return (
    <section id="cta" className="mt-20 px-2 desk:mt-[156px]">
      <Reveal className="w-full">
        <div className="flex flex-col items-center rounded-[40px] bg-gradient-to-b from-[#88b7ff] to-[#2b7fff] px-8 py-24 desk:py-[156px]">
          <div className="flex w-full max-w-[1000px] flex-col items-center gap-14">
            <Image
              src={ctaLogo}
              alt=""
              aria-hidden
              width={156}
              height={156}
              className="size-39"
            />
            <div className="flex flex-col items-center gap-4 text-center text-white">
              <h2 className="text-[28px] font-semibold leading-8 desk:text-h2 desk:leading-[60px]">
                Попробуйте Рентли в деле
              </h2>
              <p className="text-l leading-[26px] opacity-80">
                Добавьте объект и начните отслеживать аренду
              </p>
            </div>
            <LandingLink variant="white" href="/login">
              Начать бесплатно
            </LandingLink>
          </div>
        </div>
      </Reveal>
    </section>
  );
}
