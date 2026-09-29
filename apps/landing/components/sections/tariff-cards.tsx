"use client";

import Image from "next/image";
import { useEffect, useRef, useState } from "react";
import { LandingLink } from "@/components/button";
import { GLASS_GRADIENT, GLASS_INSET_SHADOW } from "@/components/glass";
import { Reveal } from "@/components/reveal";
import iconHomeMain from "@/assets/icons/icon-home-main.svg";
import iconObjects from "@/assets/icons/icon-objects.svg";
import iconTeam from "@/assets/icons/icon-team.svg";
import type { Tariff, TariffFeature } from "@/lib/content";

// «Тарифы» — макеты 2846-153711 (десктоп: 3×320×550, r40, p-40),
// 2859-4349 (планшет: ряд 3×320×453, r32, p-32), 2859-2231 (мобайл:
// стопка по контенту, r32, p-32). Заливки по макету: «Базовый» —
// светлая #F3F4F6 со стеклянным оверлеем, «Про» — тёмный градиент
// (100→30), «Бизнес» — синий (136,183,255→43,127,255); стеклянные
// карточки с белым инсет-светом и конической градиентной обводкой
// (.glass-ring); константы стекла — общие с «Для кого сервис»
// (components/glass.ts), заголовки фич — Landing/M (Medium 500).
// Переключатель 256×56: активный сегмент — белая пилюля, переезжающая
// между Год/Месяц за 300мс (iOS-сегментконтрол); бейдж «−25%» живёт
// в сегменте «Год» всегда. Цена при переключении меняется в хореографии
// steps (180мс уход → 280мс въезд, токен --animate-tariff-price-in);
// prefers-reduced-motion глушит и пилюлю, и смену цены. Кнопка «Ваш
// тариф» — White Disabled (серый текст).
const PRICE_OUT_MS = 180;

type PricePhase = "idle" | "leaving" | "entering";

const FEATURE_ICONS: Record<TariffFeature["icon"], { src: string; alt: string }> = {
  home: { src: iconHomeMain.src, alt: "Иконка объекта" },
  objects: { src: iconObjects.src, alt: "Иконка объектов" },
  team: { src: iconTeam.src, alt: "Иконка команды" },
};

const CARD_THEMES = {
  basic: {
    backgroundImage: GLASS_GRADIENT,
    backgroundColor: "#f3f4f6",
    text: "text-ink",
    sub: "text-gray-2",
    featureSub: "text-gray-2",
  },
  pro: {
    backgroundImage: `${GLASS_GRADIENT}, linear-gradient(180deg, rgb(100,100,100) 0%, rgb(30,30,30) 100%)`,
    backgroundColor: "transparent",
    text: "text-white",
    sub: "text-white/80",
    featureSub: "text-white/80",
  },
  business: {
    backgroundImage: `${GLASS_GRADIENT}, linear-gradient(180deg, rgb(136,183,255) 0%, rgb(43,127,255) 100%)`,
    backgroundColor: "transparent",
    text: "text-white",
    sub: "text-white/80",
    featureSub: "text-white/80",
  },
} as const;

type Billing = "yearly" | "monthly";

export function TariffCards({
  tariffs,
  currentTariff,
}: {
  tariffs: Tariff[];
  currentTariff?: string;
}) {
  const [billing, setBilling] = useState<Billing>("yearly");
  const [displayedBilling, setDisplayedBilling] = useState<Billing>("yearly");
  const [pricePhase, setPricePhase] = useState<PricePhase>("idle");
  const swapTimer = useRef<number | null>(null);

  useEffect(() => {
    return () => {
      if (swapTimer.current !== null) {
        window.clearTimeout(swapTimer.current);
      }
    };
  }, []);

  const selectBilling = (value: Billing) => {
    if (value === billing) {
      return;
    }
    // Пилюля уезжает сразу по клику, цена догоняет в хореографии steps.
    setBilling(value);
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
      setDisplayedBilling(value);
      setPricePhase("idle");
      return;
    }
    if (swapTimer.current !== null) {
      window.clearTimeout(swapTimer.current);
    }
    if (pricePhase === "entering") {
      // Клик посреди въезда: старая цена ещё полупрозрачна — подменяем
      // её сразу и переигрываем въезд, не гоняя лишнюю фазу ухода.
      setDisplayedBilling(value);
      return;
    }
    setPricePhase("leaving");
    swapTimer.current = window.setTimeout(() => {
      setDisplayedBilling(value);
      setPricePhase("entering");
    }, PRICE_OUT_MS);
  };

  return (
    <>
      {/* Заголовок и переключатель: на планшете/мобайле — столбиком по центру,
          на десктопе — в одну строку от левого края (макет 2846-153712). */}
      <div className="flex flex-col items-center gap-6 desk:flex-row desk:items-center desk:self-stretch desk:gap-8">
        <h2 className="text-center text-[28px] font-semibold leading-8 desk:text-h2 desk:leading-[60px]">
          Тарифы
        </h2>
        <div
          className="relative flex h-14 w-64 items-center gap-0.5 rounded-[16px] bg-surface p-0.5"
          role="tablist"
          aria-label="Период оплаты"
        >
          <span
            aria-hidden
            className="absolute inset-y-0.5 left-0.5 w-[125px] rounded-[14px] bg-white shadow-[0_2px_8px_rgba(0,0,0,0.16)] transition-transform duration-300 ease-[cubic-bezier(0.4,0,0.2,1)] motion-reduce:transition-none"
            style={{
              transform:
                billing === "yearly"
                  ? "translateX(0)"
                  : "translateX(calc(100% + 2px))",
            }}
          />
          {(
            [
              ["yearly", "Год", true],
              ["monthly", "Месяц", false],
            ] as const
          ).map(([value, label, withBadge]) => (
            <button
              key={value}
              type="button"
              role="tab"
              aria-selected={billing === value}
              onClick={() => selectBilling(value)}
              className={`relative flex h-[52px] flex-1 cursor-pointer items-center justify-center gap-2 rounded-[14px] text-xs transition-colors duration-200 outline-none focus-visible:ring-2 focus-visible:ring-primary/40 ${
                billing === value ? "text-ink" : "text-gray-2"
              }`}
            >
              {label}
              {withBadge && (
                <span className="flex items-center rounded-[6px] bg-primary px-1 py-0.5 text-xs text-white">
                  −25%
                </span>
              )}
            </button>
          ))}
        </div>
      </div>
      <div className="mt-8 flex justify-start desk:mt-14">
        <div className="grid w-full grid-cols-1 gap-3 tab:w-[984px] tab:shrink-0 tab:grid-cols-3 desk:w-full desk:grid-cols-3 desk:gap-5">
          {tariffs.map((tariff, index) => {
            const isCurrent = currentTariff === tariff.id;
            const theme = CARD_THEMES[tariff.id];
            // «Бесплатно» от периода не зависит — анимируем смену только
            // у платных карточек, константную цену не дёргаем.
            const animated = tariff.id !== "basic";
            const price =
              tariff.id === "basic"
                ? "Бесплатно"
                : displayedBilling === "yearly"
                  ? `${tariff.yearlyPerMonth} ₽ в месяц`
                  : `${tariff.monthly} ₽ в месяц`;
            return (
              <Reveal key={tariff.id} delay={index * 100} className="h-full">
                <article
                  style={{
                    backgroundImage: theme.backgroundImage,
                    backgroundColor: theme.backgroundColor,
                  }}
                  className={`glass-ring relative flex h-auto w-full flex-col justify-between gap-8 overflow-clip rounded-[32px] p-8 backdrop-blur-[10px] tab:h-[453px] tab:w-[320px] desk:h-[550px] desk:gap-12 desk:rounded-[40px] desk:p-10 ${theme.text} ${GLASS_INSET_SHADOW}`}
                >
                  <div className="flex flex-col gap-[15px]">
                    <h3 className="text-s leading-5 desk:text-r desk:leading-[22px]">
                      {tariff.name}
                    </h3>
                    <div
                      key={animated ? displayedBilling : undefined}
                      onAnimationEnd={
                        animated ? () => setPricePhase("idle") : undefined
                      }
                      className={`flex flex-col gap-2.5 ${
                        animated && pricePhase === "leaving"
                          ? "-translate-y-1 opacity-0 transition-[opacity,translate] duration-[180ms] ease-[cubic-bezier(0.4,0,0.2,1)] motion-reduce:transition-none"
                          : animated && pricePhase === "entering"
                            ? "animate-tariff-price-in motion-reduce:animate-none"
                            : ""
                      }`}
                    >
                      <p className="text-[28px] font-semibold leading-8">{price}</p>
                      {/* Строка года резервируется и в «Месяце» (invisible):
                          без неё justify-between карточки сдвигает блок фич
                          на 16px при каждом переключении. */}
                      {tariff.id !== "basic" && (
                        <p
                          className={`text-s leading-5 desk:text-r desk:leading-[22px] ${theme.sub} ${
                            displayedBilling === "yearly" ? "" : "invisible"
                          }`}
                        >
                          При оплате {tariff.yearlyTotal.toLocaleString("ru-RU")} ₽ за год
                        </p>
                      )}
                    </div>
                  </div>
                  <div className="flex flex-col gap-6">
                    {tariff.features.map((feature) => (
                      <div key={feature.title} className="flex items-start gap-4">
                        <Image
                          src={FEATURE_ICONS[feature.icon].src}
                          alt={FEATURE_ICONS[feature.icon].alt}
                          width={32}
                          height={32}
                          className="size-8"
                        />
                        <div className="flex flex-col gap-1">
                          <p className="text-m font-medium leading-6">{feature.title}</p>
                          <p className={`text-xs leading-[18px] ${theme.featureSub}`}>
                            {feature.text}
                          </p>
                        </div>
                      </div>
                    ))}
                  </div>
                  {isCurrent ? (
                    <LandingLink
                      variant="white"
                      href={tariff.href}
                      aria-disabled
                      className="pointer-events-none w-full text-gray-2!"
                    >
                      Ваш тариф
                    </LandingLink>
                  ) : (
                    <LandingLink variant="white" href={tariff.href} className="w-full">
                      {tariff.cta}
                    </LandingLink>
                  )}
                </article>
              </Reveal>
            );
          })}
        </div>
      </div>
    </>
  );
}
