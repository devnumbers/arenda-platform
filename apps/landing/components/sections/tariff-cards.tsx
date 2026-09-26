"use client";

import Image from "next/image";
import { useState } from "react";
import { LandingLink } from "@/components/button";
import { Reveal } from "@/components/reveal";
import iconHomeMain from "@/assets/icons/icon-home-main.svg";
import iconObjects from "@/assets/icons/icon-objects.svg";
import iconTeam from "@/assets/icons/icon-team.svg";
import type { Tariff, TariffFeature } from "@/lib/content";

// «Тарифы» — макет 2846-153710: H2 + переключатель «Год (−25%) / Месяц»
// (256×56, сегменты 125×52), три карточки h-550 (radius-40, border
// rgba(156,156,156,.5), p-40, justify-between): имя 22 + цена 32 + подпись
// 14, фичи (иконка 32 + текст, шаг 24), кнопка на всю ширину. Для
// авторизованного кнопка его тарифа — недоступная «Ваш тариф» (2864-5099).
const FEATURE_ICONS: Record<TariffFeature["icon"], { src: string; alt: string }> = {
  home: { src: iconHomeMain.src, alt: "Иконка объекта" },
  objects: { src: iconObjects.src, alt: "Иконка объектов" },
  team: { src: iconTeam.src, alt: "Иконка команды" },
};

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
        className="flex h-14 w-64 items-center self-center rounded-[16px] bg-surface p-0.5"
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
            className={`flex h-[52px] flex-1 items-center justify-center gap-1.5 rounded-[12px] text-xs transition-all duration-200 outline-none focus-visible:ring-2 focus-visible:ring-primary/40 ${
              billing === value ? "bg-white shadow-[0_4px_16px_rgba(0,0,0,0.04)]" : ""
            }`}
          >
            {label}
            {withBadge && (
              <span className="flex h-[22px] items-center rounded-[6px] bg-primary-light px-1 text-xs text-primary">
                −25%
              </span>
            )}
          </button>
        ))}
      </div>
      <div className="grid gap-5 desk:grid-cols-3">
        {tariffs.map((tariff, index) => {
          const isCurrent = currentTariff === tariff.id;
          const price =
            tariff.id === "basic"
              ? "Бесплатно"
              : billing === "yearly"
                ? `${tariff.yearlyPerMonth} ₽ в месяц`
                : `${tariff.monthly} ₽ в месяц`;
          return (
            <Reveal key={tariff.id} delay={index * 100} className="h-full">
              <article className="flex h-[550px] flex-col justify-between rounded-[40px] border border-[rgba(156,156,156,0.5)] p-10">
                <div className="flex flex-col gap-2.5">
                  <h3 className="text-r leading-[22px]">{tariff.name}</h3>
                  <div className="flex flex-col gap-2.5">
                    <p className="text-h4 font-semibold leading-8">{price}</p>
                    {billing === "yearly" && tariff.id !== "basic" && (
                      <p className="text-xs leading-[18px] text-gray-2">
                        При оплате {tariff.yearlyTotal.toLocaleString("ru-RU")} ₽ за год
                      </p>
                    )}
                  </div>
                </div>
                <div className="flex flex-col gap-6">
                  {tariff.features.map((feature) => (
                    <div key={feature.title} className="flex items-start gap-3">
                      <Image
                        src={FEATURE_ICONS[feature.icon].src}
                        alt={FEATURE_ICONS[feature.icon].alt}
                        width={32}
                        height={32}
                        className="size-8"
                      />
                      <div className="flex flex-col gap-1.5">
                        <p className="text-r leading-6">{feature.title}</p>
                        <p className="text-xs leading-[18px] text-gray-2">
                          {feature.text}
                        </p>
                      </div>
                    </div>
                  ))}
                </div>
                {isCurrent ? (
                  <LandingLink
                    variant="gray"
                    href={tariff.href}
                    aria-disabled
                    className="pointer-events-none w-full opacity-100"
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
    </>
  );
}
