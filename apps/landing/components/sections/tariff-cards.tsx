"use client";

import Image from "next/image";
import { useState } from "react";
import { LandingLink } from "@/components/button";
import { Reveal } from "@/components/reveal";
import iconHomeMain from "@/assets/icons/icon-home-main.svg";
import iconObjects from "@/assets/icons/icon-objects.svg";
import iconTeam from "@/assets/icons/icon-team.svg";
import type { Tariff, TariffFeature } from "@/lib/content";

// «Тарифы» — макеты 2846-153710 (десктоп: 3×320×550, r40, p-40),
// 2859-4349 (планшет: ряд 3×320×453, r32, p-32), 2859-2231 (мобайл:
// стопка по контенту, r32, p-32). Заливки по макету: «Базовый» —
// светлая #F3F4F6, «Про» — тёмный градиент (100→30), «Бизнес» — синий
// (136,183,255→43,127,255); стеклянные карточки с белым инсет-светом.
// Переключатель 256×56: активный сегмент белый с тенью, бейдж «−25%» —
// сплошной синий; кнопка «Ваш тариф» — White Disabled (серый текст).
const FEATURE_ICONS: Record<TariffFeature["icon"], { src: string; alt: string }> = {
  home: { src: iconHomeMain.src, alt: "Иконка объекта" },
  objects: { src: iconObjects.src, alt: "Иконка объектов" },
  team: { src: iconTeam.src, alt: "Иконка команды" },
};

const GLASS_INSET_SHADOW =
  "shadow-[inset_0px_-5px_4px_0px_rgba(255,255,255,0.25),inset_0px_4px_4px_0px_rgba(255,255,255,0.25)]";

const CARD_THEMES = {
  basic: {
    background: "#f3f4f6",
    text: "text-ink",
    sub: "text-gray-2",
    featureSub: "text-gray-2",
  },
  pro: {
    background:
      "linear-gradient(180deg, rgba(255,255,255,0.1) 0%, rgba(217,217,217,0.1) 100%), linear-gradient(180deg, rgb(100,100,100) 0%, rgb(30,30,30) 100%)",
    text: "text-white",
    sub: "text-white/80",
    featureSub: "text-white/80",
  },
  business: {
    background:
      "linear-gradient(180deg, rgba(255,255,255,0.1) 0%, rgba(217,217,217,0.1) 100%), linear-gradient(180deg, rgb(136,183,255) 0%, rgb(43,127,255) 100%)",
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

  return (
    <>
      <div
        className="flex h-14 w-64 items-center self-center gap-0.5 rounded-[16px] bg-surface p-0.5"
        role="tablist"
        aria-label="Период оплаты"
      >
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
            onClick={() => setBilling(value)}
            className={`flex h-[52px] flex-1 items-center justify-center gap-2 rounded-[14px] text-xs transition-all duration-200 outline-none focus-visible:ring-2 focus-visible:ring-primary/40 ${
              billing === value
                ? "bg-white text-ink shadow-[0_2px_8px_rgba(0,0,0,0.16)]"
                : "text-gray-2"
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
      <div className="mt-8 flex justify-center">
        <div className="grid w-full grid-cols-1 gap-3 tab:w-[984px] tab:shrink-0 tab:grid-cols-3 desk:w-full desk:grid-cols-3 desk:gap-5">
          {tariffs.map((tariff, index) => {
            const isCurrent = currentTariff === tariff.id;
            const theme = CARD_THEMES[tariff.id];
            const price =
              tariff.id === "basic"
                ? "Бесплатно"
                : billing === "yearly"
                  ? `${tariff.yearlyPerMonth} ₽ в месяц`
                  : `${tariff.monthly} ₽ в месяц`;
            return (
              <Reveal key={tariff.id} delay={index * 100} className="h-full">
                <article
                  style={{ backgroundImage: theme.background }}
                  className={`relative flex h-auto w-full flex-col justify-between gap-12 overflow-clip rounded-[32px] border border-[rgba(156,156,156,0.5)] p-8 backdrop-blur-[10px] tab:h-[453px] tab:w-[320px] desk:h-[550px] desk:rounded-[40px] desk:p-10 ${theme.text} ${GLASS_INSET_SHADOW}`}
                >
                  <div className="flex flex-col gap-[15px]">
                    <h3 className="text-s leading-5 desk:text-r desk:leading-[22px]">
                      {tariff.name}
                    </h3>
                    <div className="flex flex-col gap-2.5">
                      <p className="text-[28px] font-semibold leading-8">{price}</p>
                      {billing === "yearly" && tariff.id !== "basic" && (
                        <p className={`text-s leading-5 ${theme.sub}`}>
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
                          <p className="text-m leading-6">{feature.title}</p>
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
