import Image from "next/image";
import { Reveal } from "@/components/reveal";
import sharingPhoto1 from "@/assets/sections/sharing-photo-1.webp";
import sharingPhoto2 from "@/assets/sections/sharing-photo-2.webp";

// «Делитесь объектом» — макет 2814-951: H2 56 + подпись 28 Regular (левый
// блок), затем голубая панель (#2b7fff1a, radius-40, h-450, p-40) с двумя
// фото: 400×400 radius-40 c синей тенью слева сверху, 380×380 справа снизу.
export function Sharing() {
  return (
    <section id="sharing" className="mt-20 desk:mt-[156px]">
      <div className="mx-auto w-full max-w-[1000px] px-5 desk:px-10">
        <Reveal>
          <div className="flex flex-col gap-6">
            <h2 className="text-[28px] font-semibold leading-8 desk:text-h2 desk:leading-[60px]">
              Делитесь объектом
            </h2>
            <p className="text-r desk:text-[28px] desk:leading-8 desk:text-gray-2">
              Приглашайте партнеров и управляйте объектом вместе
            </p>
          </div>
        </Reveal>
        <Reveal delay={100}>
          {/* Десктоп — абсолютная композиция по макету; мобайл — два фото рядом. */}
          <div className="relative mt-14 flex min-h-[420px] items-end justify-center gap-4 rounded-[40px] bg-primary-light p-8 desk:mt-14 desk:block desk:h-[450px] desk:p-10">
            <Image
              src={sharingPhoto2}
              alt="Партнеры работают с объектом вместе"
              sizes="(min-width: 1200px) 400px, 45vw"
              className="hidden h-[400px] w-[400px] rounded-[40px] shadow-[0_8px_24px_rgba(43,127,255,0.08)] desk:absolute desk:left-[72px] desk:top-[72px] desk:block"
            />
            <Image
              src={sharingPhoto1}
              alt="Совместный доступ к объекту в Рентли"
              sizes="(min-width: 1200px) 380px, 45vw"
              className="h-[300px] w-[300px] rounded-[40px] object-cover desk:absolute desk:bottom-0 desk:left-[542px] desk:h-[380px] desk:w-[380px]"
            />
          </div>
        </Reveal>
      </div>
    </section>
  );
}
