"use client";

import Image from "next/image";
import {
  BROWSERS,
  InstallModal,
  useInstallModal,
  type BrowserId,
} from "@/components/install-instructions";
import contactBg from "@/assets/sections/contact-bg.webp";
import contactIphone from "@/assets/sections/contact-iphone.webp";

// PWA-блок — макеты 3343-160903 (десктоп: карточка 1000×450, r40, p-52:
// шапка сверху, «Как установить для» + кнопки браузеров снизу, айфон
// 406 у низа справа), 3005:78366 (планшет: r40, p-52, колонка 350,
// заголовок 32/36 SemiBold, айфон 458 у правого края) и 3343-160963
// (мобайл: r32, p-32, айфон 302 снизу справа). Новый текст подписи и
// блок «Как установить для» — кнопки-чипы браузеров (белые, r16,
// pl-28/pr-24, иконка 32; на планшете 16/20 h-56, на десктопе 18/22
// h-64), каждая открывает модалку инструкции (install-instructions.tsx;
// фаза закрытия модалки — тот же владелец состояния, useInstallModal).
const BROWSER_ORDER: BrowserId[] = ["chrome", "yandex", "safari"];

export function Contact() {
  const { browser, closing, open, close } = useInstallModal();

  return (
    <section id="contact" className="mt-24 scroll-mt-[88px] desk:mt-[156px] desk:scroll-mt-[104px]">
      <div className="mx-auto w-full max-w-[1048px] px-6 desk:max-w-[1000px] desk:px-0">
        <div className="w-full">
          <div className="relative flex h-[700px] flex-col gap-8 overflow-clip rounded-[32px] p-8 tab:h-[600px] tab:flex-row tab:rounded-[40px] tab:p-[52px] desk:h-[450px]">
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
            <div className="relative flex w-full flex-col gap-8 tab:h-full tab:w-[350px] tab:shrink-0 tab:justify-between desk:w-auto desk:flex-1">
              <div className="flex flex-col gap-3 desk:gap-6">
                {/* Переносы явные только на планшете: браузерный Onest уже
                    фиргинского (паттерн прежнего блока; баланс строк —
                    глобально в globals.css, text-wrap-style: balance). */}
                <p className="text-[28px] font-semibold leading-8 tab:text-[32px] tab:leading-9 desk:text-h3 desk:font-medium desk:leading-10">
                  Установите сайт
                  <br className="hidden tab:block desk:hidden" /> как приложение
                </p>
                <p className="text-s leading-5 text-ink opacity-60 desk:text-r desk:leading-[22px]">
                  Доступно для Google Chrome, Яндекс Бразера, Safari и других
                  браузеров. Для компьютеров и телефонов
                </p>
              </div>
              <div className="flex flex-col gap-5">
                <p className="text-l leading-[26px]">Как установить для</p>
                <div className="flex flex-wrap gap-2">
                  {BROWSER_ORDER.map((id) => (
                    <button
                      key={id}
                      type="button"
                      onClick={() => open(id)}
                      className="flex min-h-14 cursor-pointer items-center gap-2 rounded-2xl bg-white pl-7 pr-6 text-s leading-5 text-ink transition-colors duration-200 active:duration-75 outline-none select-none hover:bg-surface-hover active:bg-surface-active focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary desk:min-h-16 desk:text-r desk:leading-[22px]"
                    >
                      {BROWSERS[id].label}
                      <Image
                        src={BROWSERS[id].icon}
                        alt=""
                        aria-hidden
                        width={32}
                        height={32}
                        className="size-8"
                      />
                    </button>
                  ))}
                </div>
              </div>
            </div>
            <Image
              src={contactIphone}
              alt="Рентли как приложение на iPhone"
              width={406}
              height={406}
              sizes="(min-width: 1200px) 406px, (min-width: 481px) 458px, 302px"
              /* Бокс картинки заходит на зону кнопок браузеров (прозрачная
                  часть мокапа) — без pointer-events-none он глотает клики. */
              className="pointer-events-none absolute right-0 top-[409px] h-[302px] w-[302px] tab:top-[191px] tab:h-[458px] tab:w-[458px] desk:bottom-0 desk:top-auto desk:h-[406px] desk:w-[406px]"
            />
          </div>
        </div>
      </div>
      {browser !== null && (
        <InstallModal browser={browser} closing={closing} onClose={close} />
      )}
    </section>
  );
}
