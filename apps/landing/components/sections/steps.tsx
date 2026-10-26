"use client";

import Image from "next/image";
import { useEffect, useRef, useState } from "react";
import Swiper, { type Swiper as SwiperType } from "swiper";
import { EffectFade } from "swiper/modules";
import "swiper/css";
import "swiper/css/effect-fade";
import { LandingLink } from "@/components/button";
import { STEPS } from "@/lib/content";
import s from "./steps.module.css";

// «Как начать пользоваться» — макеты 2967-75958 (десктоп: подложка во
// всю ширину блока, r40, тень низа — слой 2967-75969; прежний десктоп
// 2859-2531 с полями 8 заменён решением владельца 24.10), 2859-3647
// (планшет), 2826-152026 (мобайл): градиентная подложка (прозрачный →
// синий 10%, контент прижат к низу — на всех ярусах во всю ширину
// страницы; тень низа — на всех ярусах, решение владельца 24.10),
// H2 + 4 круглых StepButton 44×44, мокап телефона 273×400; синяя
// кнопка — ПОД подложкой (шаг 32, десктоп 56).
//
// Механика — полная копия эталона pay.yandex.ru/business/payments/qr,
// секция «Как подключить QR-код к вашему бизнесу» (живой замер 26.10,
// канон в DESIGN.md §3): один Swiper, эффект fade cross-fade — слайд
// «текст шага + мокап» целиком кроссфейдится на месте, opacity ведёт
// жест (progress слайдов); свайп и мышиный драг по всей зоне карусели
// (simulateTouch как в эталоне, grabCursor выключен), петли нет — за
// краем сопротивление и возврат ядра; автоплея нет (у эталона автоплей
// прописан в конфиге, но фактически не стартует; решение владельца
// 26.10), клавиатурных стрелок нет. Скорости эталона: 360мс при окне
// ≥720 и 460мс при <720; breakpoints ядра меряют контейнер, а граница
// эталона живёт на ширине окна (канон ярусов DESIGN.md §2), поэтому
// скорость ставится вручную по matchMedia; кривая — дефолт ядра CSS
// ease (как в эталоне).
//
// Кнопки шагов — вне карусели, как буллеты пагинации эталона: клик
// ведёт slideTo, активная следует за slideChange (перекраска фона
// 200мс — как 0.2s ease буллетов эталона). Роль group и aria-pressed —
// прежняя семантика секции (панельной семантики — стрелки, roving
// tabindex, tabpanel — нет).
//
// Текстовая зона слайда — min-h на три строки: в fade слайды лежат
// стопкой в точке активного (ядро контр-транслейтит), без равной
// высоты мокапы шагов прыгали бы по вертикали при кроссфейде; у
// эталона слайды тоже равной высоты (inline height 534px, замер).
//
// Картинки шагов — eager (канон фото DESIGN.md §4): в fade слайды
// стоят в ряду по layout за вьюпортом, и нативная ленивая загрузка по
// layout-позиции их не триггерит (живой замер на стенде 26.10:
// запросов нет, пока не форснуть eager); у эталона картинки слайдов
// тоже loading="eager". Все четыре грузятся сразу — как в прежней
// версии секции, где стопка мокапов лежала в одной точке и ленилась
// как видимая.

// Граница — ширина окна из эталона (720), не ярус лендинга desk:
// (1200), поэтому имена нейтральные — wide/narrow, не desk/mob.
const SPEED_WIDE_MS = 360; // эталон: окно ≥720
const SPEED_NARROW_MS = 460; // эталон: окно <720

export function Steps() {
  const [active, setActive] = useState(0);
  const carouselRef = useRef<HTMLDivElement>(null);
  const swRef = useRef<SwiperType | null>(null);

  useEffect(() => {
    const el = carouselRef.current;
    if (!el) return;
    const mmMob = window.matchMedia("(max-width: 719px)");
    const speedOf = () => (mmMob.matches ? SPEED_NARROW_MS : SPEED_WIDE_MS);
    const sw = new Swiper(el, {
      modules: [EffectFade],
      effect: "fade",
      fadeEffect: { crossFade: true },
      // мышиный драг на десктопе — решение владельца 26.10 (как в
      // эталоне); дефолт ядра true, ставим явно против смены дефолтов
      simulateTouch: true,
      // progress слайдов ведёт opacity кроссфейда: без него opacity
      // меняется только на программных переходах, не в жесте
      watchSlidesProgress: true,
      speed: speedOf(),
      on: {
        slideChange: (swiper: SwiperType) => setActive(swiper.realIndex),
      },
    });
    swRef.current = sw;
    const onBoundary = () => {
      if (!sw.destroyed) sw.params.speed = speedOf();
    };
    mmMob.addEventListener("change", onBoundary);
    return () => {
      mmMob.removeEventListener("change", onBoundary);
      sw.destroy(true, true);
      if (swRef.current === sw) swRef.current = null;
    };
  }, []);

  const selectStep = (index: number) => {
    if (index === active) {
      return;
    }
    swRef.current?.slideTo(index);
  };

  return (
    <section id="steps" className="mt-24 scroll-mt-[88px] desk:mt-[156px] desk:scroll-mt-[104px]">
      <div className="flex flex-col items-center gap-8 desk:gap-14">
        <div className="relative flex w-full flex-col items-center justify-end overflow-clip rounded-[32px] desk:rounded-[40px] bg-[linear-gradient(180deg,rgba(43,127,255,0)_0%,rgba(43,127,255,0.1)_100%)] px-8 desk:px-10">
          <div className="w-full">
            <div className="flex w-full flex-col items-center gap-6 desk:gap-8">
              <h2 className="text-center text-[28px] font-semibold leading-8 tab:text-[32px] tab:leading-9 desk:text-h2 desk:leading-[60px]">
                Как начать пользоваться
              </h2>
              <div className="flex items-center gap-4" role="group" aria-label="Шаги">
                {STEPS.map((item, index) => (
                  <button
                    key={item.text}
                    type="button"
                    aria-pressed={active === index}
                    aria-label={`Шаг ${index + 1}`}
                    onClick={() => selectStep(index)}
                    className={`flex size-11 cursor-pointer items-center justify-center rounded-[100px] text-xl font-bold transition-colors duration-200 active:duration-75 outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary [text-shadow:0_8px_24px_rgba(43,127,255,0.08)] ${
                      active === index
                        ? "bg-primary text-white"
                        : "bg-primary-light text-primary-disabled hover:text-primary active:text-primary-active"
                    }`}
                  >
                    {index + 1}
                  </button>
                ))}
              </div>
            </div>
          </div>
          {/* Слот 24 / десктоп 32 — прежняя дистанция кнопки → текст
              (gap-6/desk:gap-8 бывшей колонки: текст теперь внутри
              слайда, карусель заняла его место). */}
          <div
            ref={carouselRef}
            className={`swiper swiper-fade swiper-horizontal ${s.zone} mt-6 w-full select-none desk:mt-8`}
          >
            <div className="swiper-wrapper">
              {STEPS.map((item, index) => (
                <div key={item.text} className="swiper-slide" aria-hidden={active !== index}>
                  <p className="mx-auto min-h-[60px] max-w-[250px] text-center text-s leading-5 text-ink/50 tab:max-w-none desk:min-h-[78px] desk:text-l desk:leading-[26px]">
                    {item.text}
                  </p>
                  <div className="mt-14 flex justify-center">
                    <Image
                      src={item.image}
                      alt={item.alt}
                      width={273}
                      height={400}
                      sizes="273px"
                      loading="eager"
                      draggable={false}
                    />
                  </div>
                </div>
              ))}
            </div>
          </div>
          {/* Тень низа панели — слой 2967-75969 из макета 2967-75958:
              градиент #F3F4F6 2% → #171A1C 2% на высоту 128 от низа,
              поверх контента (z-20 — над мокапом, в макете слой
              самый верхний); ширина — вся ширина панели (в макете слой
              тянется за контекстом, не фикс), на всех ярусах (решение
              владельца 24.10). */}
          <div
            aria-hidden
            className="pointer-events-none absolute inset-x-0 bottom-0 z-20 h-32 bg-[linear-gradient(180deg,rgba(243,244,246,0.02)_0%,rgba(23,26,28,0.02)_100%)]"
          />
        </div>
        <LandingLink href="/login">Начать пользоваться</LandingLink>
      </div>
    </section>
  );
}
