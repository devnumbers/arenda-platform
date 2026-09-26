import Image from "next/image";
import { Reveal } from "@/components/reveal";
import sharingPhoto1 from "@/assets/sections/sharing-photo-1.webp";
import sharingPhoto2 from "@/assets/sections/sharing-photo-2.webp";

// «Делитесь объектом» — макеты 2814-951 (десктоп: заголовок слева, панель
// r40 h-450, карточка 400 на (72,72), пара 380 справа снизу), 2859-3561
// (планшет: всё по центру, панель r32 h-450, карточка 326, пара 380),
// 2826-151461 (мобайл: панель r32 h-500, карточка 223 r24, пара 260).
// Панель — primary на 10% альфы (по живому канвасу Figma, 26.09).
export function Sharing() {
  return (
    <section id="sharing" className="mt-24 desk:mt-[156px]">
      <div className="mx-auto w-full max-w-[1048px] px-6 desk:max-w-[1000px] desk:px-0">
        <Reveal>
          <div className="flex flex-col items-center gap-3 text-center desk:items-start desk:gap-6 desk:text-left">
            <h2 className="text-[28px] font-semibold leading-8 desk:text-h2 desk:leading-[60px]">
              Делитесь объектом
            </h2>
            <p className="text-s leading-5 text-gray-2 desk:text-[28px] desk:leading-8">
              Приглашайте партнеров и управляйте объектом вместе
            </p>
          </div>
        </Reveal>
        <Reveal delay={100}>
          {/* Десктоп — абсолютная композиция по макету; планшет/мобайл — свои
              размеры и позиции фото (в процентах от ширины панели). */}
          <div className="relative mt-8 h-[500px] overflow-hidden rounded-[32px] bg-primary-light desk:mt-14 desk:block desk:h-[450px] desk:rounded-[40px]">
            <Image
              src={sharingPhoto2}
              alt="Партнеры работают с объектом вместе"
              sizes="(min-width: 1200px) 400px, (min-width: 481px) 326px, 223px"
              className="absolute left-[17.5%] top-[45px] h-[223px] w-[223px] rounded-[24px] shadow-[0_8px_24px_rgba(43,127,255,0.08)] tab:left-[8%] tab:top-[62px] tab:h-[326px] tab:w-[326px] tab:rounded-[32px] desk:left-[72px] desk:top-[72px] desk:h-[400px] desk:w-[400px] desk:rounded-[40px]"
            />
            <Image
              src={sharingPhoto1}
              alt="Совместный доступ к объекту в Рентли"
              sizes="(min-width: 1200px) 380px, (min-width: 481px) 380px, 260px"
              className="absolute left-[12%] top-[240px] h-[260px] w-[260px] tab:left-[41%] tab:top-[70px] tab:h-[380px] tab:w-[380px] desk:bottom-0 desk:left-[542px] desk:top-auto desk:h-[380px] desk:w-[380px]"
            />
          </div>
        </Reveal>
      </div>
    </section>
  );
}
