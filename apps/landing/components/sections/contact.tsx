import Image from "next/image";
import { Reveal } from "@/components/reveal";
import contactBg from "@/assets/sections/contact-bg.webp";
import contactIphone from "@/assets/sections/contact-iphone.webp";

// PWA-блок — макеты 2814-1135 (десктоп: карточка 406, r40, p-52, текст
// 36/40 Medium + подпись 18/22 с шагом 156, айфон 406 у верха справа),
// 2859:3803 (планшет: 500, r40, p-52, текст 28/32 SemiBold + 16/20 с
// шагом 12, айфон 458 у правого края) и 2826:151647 (мобайл: 550, r32,
// p-32, айфон 302 снизу справа).
export function Contact() {
  return (
    <section id="contact" className="mt-24 scroll-mt-[88px] desk:mt-[156px] desk:scroll-mt-[104px]">
      <div className="mx-auto w-full max-w-[1048px] px-6 desk:max-w-[1000px] desk:px-0">
        <Reveal className="w-full">
          <div className="relative flex h-[550px] items-start justify-between overflow-clip rounded-[32px] p-8 tab:h-[500px] tab:rounded-[40px] tab:p-[52px] desk:h-[406px] desk:p-[52px]">
            <div className="pointer-events-none absolute inset-0 overflow-hidden rounded-[32px] tab:rounded-[40px]">
              <Image
                src={contactBg}
                alt=""
                aria-hidden
                fill
                sizes="1000px"
                className="object-cover"
              />
            </div>
            <div className="relative flex w-full max-w-[260px] flex-col gap-3 desk:max-w-[290px] desk:gap-[156px]">
              <p className="text-[28px] font-semibold leading-8 desk:text-h3 desk:font-medium desk:leading-10">
                Установите сайт как приложение
              </p>
              <p className="text-s leading-5 text-ink opacity-60 desk:text-r desk:leading-[22px]">
                Доступно для браузеров Google Chrome, Яндекс Бразера, Safari,
                для компьютеров и телефонов
              </p>
            </div>
            <Image
              src={contactIphone}
              alt="Рентли как приложение на iPhone"
              width={406}
              height={406}
              sizes="(min-width: 1200px) 406px, (min-width: 481px) 458px, 302px"
              className="absolute right-0 top-[259px] h-[302px] w-[302px] tab:top-[185px] tab:h-[458px] tab:w-[458px] desk:right-0 desk:top-0 desk:h-[406px] desk:w-[406px]"
            />
          </div>
        </Reveal>
      </div>
    </section>
  );
}
