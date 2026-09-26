import Image from "next/image";
import { Reveal } from "@/components/reveal";
import contactBg from "@/assets/sections/contact-bg.webp";
import contactIphone from "@/assets/sections/contact-iphone.webp";

// PWA-блок — макет 2814-1135: карточка radius-40 с фоном-градиентом
// (растянутый блюр-фон макета), слева текст (заголовок 36/40 Medium сверху,
// подпись 18/22 opacity-60 снизу, шаг 156), справа мокап iPhone 406×406.
export function Contact() {
  return (
    <section id="contact" className="mt-20 desk:mt-[156px]">
      <div className="mx-auto w-full max-w-[1000px] px-5 desk:px-10">
        <Reveal className="w-full">
          <div className="relative flex min-h-[420px] items-start justify-between overflow-clip rounded-[40px] p-8 desk:min-h-[510px] desk:p-[52px]">
            <div className="pointer-events-none absolute inset-0 overflow-hidden rounded-[40px]">
              <Image
                src={contactBg}
                alt=""
                aria-hidden
                fill
                sizes="1000px"
                className="object-cover"
              />
            </div>
            <div className="relative flex w-[342px] max-w-full flex-col gap-24">
              <p className="text-h3 font-medium leading-10">
                Установите сайт как приложение
              </p>
              <p className="text-r leading-[22px] text-ink opacity-60">
                Доступно для браузеров Google Chrome, Яндекс Бразера, Safari,
                для компьютеров и телефонов
              </p>
            </div>
            <Image
              src={contactIphone}
              alt="Рентли как приложение на iPhone"
              width={406}
              height={406}
              sizes="(min-width: 1200px) 406px, 45vw"
              className="relative hidden w-[406px] desk:absolute desk:right-0 desk:top-1/2 desk:block desk:-translate-y-1/2"
            />
          </div>
        </Reveal>
      </div>
    </section>
  );
}
