import Image from "next/image";
import { LandingLink } from "@/components/button";
import { Reveal } from "@/components/reveal";
import ctaLogo from "@/assets/sections/cta-logo-3d.webp";

// Финальный CTA — макеты 2814-1142 (десктоп: панель r40 с полями 8,
// py-156) и 2859:3809 / 2826:151653 (планшет/мобайл: панель во всю
// ширину страницы, r40, py-96): логотип-3D 156×156, H2 28 белым,
// подпись 16/20 white/80 (десктоп 56 и 22/26), белая кнопка. Отступ
// до футера по фулл-фреймам 2814-729 / 2859-3454 / 2826-151222 —
// 156 на десктопе и 96 на планшете/мобайле; футер общий с
// юрстраницами, поэтому зазор живёт здесь, а не на нём.
export function Cta() {
  return (
    <section
      id="cta"
      className="mt-24 mb-24 desk:mt-[156px] desk:mb-[156px] desk:px-2"
    >
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
              <h2 className="text-balance text-[28px] font-semibold leading-8 desk:text-h2 desk:leading-[60px]">
                Попробуйте Рентли в деле
              </h2>
              <p className="text-s leading-5 opacity-80 desk:text-l desk:leading-[26px]">
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
