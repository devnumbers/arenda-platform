import Image from "next/image";
import { Reveal } from "@/components/reveal";
import sharingPhoto1 from "@/assets/sections/sharing-photo-1.webp";
import sharingPhoto2 from "@/assets/sections/sharing-photo-2.webp";

// «Делитесь объектом» — макеты 2814-951 (десктоп: заголовок по центру,
// панель r40 h-450, карточка 400 на (72,72) выступает за низ панели),
// 2859-3561 (планшет 768: панель 720 r32 h-450, карточка 326 на left 8%
// top 62, пара 380 вплотную к низу и в 43px от правого края), 2826-151462
// (мобайл: панель r32 h-500, карточка 223 r24 по центру, top 45, пара 260
// по центру, вплотную к низу). Планшет: левый край карточки 8% и правый
// край пары 6% от ширины панели, ширины min(макетный px, % панели) —
// на 768 пиксельно по макету, ниже композиция сжимается вместе с панелью
// и не вылезает на всём диапазоне 481–1199. Мобайл: картинки центрируются
// left-1/2 — проценты, откалиброванные под 393, гуляли от центра на
// остальных ширинах. Панель — primary на 10% альфы (по живому канвасу
// Figma, 26.09); overflow-hidden нет — карточка десктопа выступает ниже
// панели, как в макете.
export function Sharing() {
  return (
    <section id="sharing" className="mt-24 scroll-mt-[88px] desk:mt-[156px] desk:scroll-mt-[104px]">
      <div className="mx-auto w-full max-w-[1048px] px-6 desk:max-w-[1000px] desk:px-0">
        <Reveal>
          <div className="flex flex-col items-center gap-3 text-center">
            <h2 className="text-[28px] font-semibold leading-8 tab:text-[32px] tab:leading-9 desk:text-h2 desk:leading-[60px]">
              Делитесь объектом
            </h2>
            <p className="text-s leading-5 text-gray-2 [text-shadow:0_8px_24px_rgba(0,0,0,0.12)] desk:text-[28px] desk:leading-8">
              Приглашайте партнеров и управляйте объектом вместе
            </p>
          </div>
        </Reveal>
        <Reveal delay={100}>
          {/* Планшет/мобайл — свои размеры и позиции фото (в процентах от
              ширины панели); на десктопе — абсолютная композиция по макету. */}
          <div className="relative mt-8 h-[500px] rounded-[32px] bg-primary-light tab:h-[450px] desk:mt-14 desk:h-[450px] desk:rounded-[40px]">
            <Image
              src={sharingPhoto2}
              alt="Партнеры работают с объектом вместе"
              sizes="(min-width: 1200px) 400px, (min-width: 481px) 326px, 223px"
              className="absolute left-1/2 top-[45px] h-[223px] w-[223px] -translate-x-1/2 rounded-[24px] shadow-[0_8px_24px_rgba(43,127,255,0.08)] tab:left-[8%] tab:top-[62px] tab:h-auto tab:w-[min(326px,45.3%)] tab:translate-x-0 tab:rounded-[32px] desk:left-[72px] desk:top-[72px] desk:h-[400px] desk:w-[400px] desk:rounded-[40px]"
            />
            <Image
              src={sharingPhoto1}
              alt="Совместный доступ к объекту в Рентли"
              sizes="(min-width: 1200px) 380px, (min-width: 481px) 380px, 260px"
              className="absolute bottom-0 left-1/2 h-[260px] w-[260px] -translate-x-1/2 tab:left-auto tab:right-[6%] tab:h-auto tab:w-[min(380px,52.8%)] tab:translate-x-0 desk:left-[542px] desk:right-auto desk:h-[380px] desk:w-[380px]"
            />
          </div>
        </Reveal>
      </div>
    </section>
  );
}
