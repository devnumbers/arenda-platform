"use client";

import Image from "next/image";
import { useState } from "react";
import { LandingLink } from "@/components/button";
import {
  GLASS_FILL_INK,
  GLASS_FILL_LIGHT,
  GLASS_FILL_PRIMARY,
  GLASS_GRADIENT,
  GLASS_INSET_SHADOW,
} from "@/components/glass";
import { Reveal } from "@/components/reveal";
import { TabletStrip } from "@/components/tablet-strip";
import { useSwapPhase } from "@/components/use-swap-phase";
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

const FEATURE_ICONS: Record<TariffFeature["icon"], { src: string; alt: string }> = {
  home: { src: iconHomeMain.src, alt: "Иконка объекта" },
  objects: { src: iconObjects.src, alt: "Иконка объектов" },
  team: { src: iconTeam.src, alt: "Иконка команды" },
};

const CARD_THEMES = {
  basic: {
    backgroundImage: GLASS_GRADIENT,
    backgroundColor: GLASS_FILL_LIGHT,
    text: "text-ink",
    sub: "text-gray-2",
    featureSub: "text-gray-2",
  },
  pro: {
    backgroundImage: `${GLASS_GRADIENT}, ${GLASS_FILL_INK}`,
    backgroundColor: "transparent",
    text: "text-white",
    sub: "text-white/80",
    featureSub: "text-white/80",
  },
  business: {
    backgroundImage: `${GLASS_GRADIENT}, ${GLASS_FILL_PRIMARY}`,
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
  const {
    phase: pricePhase,
    value: displayedBilling,
    swap: swapPrice,
    settle: settlePrice,
  } = useSwapPhase<Billing>({ initial: "yearly", outMs: PRICE_OUT_MS });

  const selectBilling = (value: Billing) => {
    if (value === billing) {
      return;
    }
    // Пилюля уезжает сразу по клику, цена догоняет в хореографии steps.
    setBilling(value);
    swapPrice(value);
  };

  // Карточки — общие для нативной сетки (мобайл/десктоп) и активной
  // планшетной полосы: движок меряет шаг позиций покоя по прямым детям
  // дорожки, поэтому в активном режиме карточки идут в неё напрямую,
  // без сетки-обёртки (иначе снап и бросок перескакивали бы по две
  // карточки на узких планшетах, где ход длиннее шага).
  const cards = tariffs.map((tariff, index) => {
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
      <Reveal
        key={tariff.id}
        delay={index * 100}
        className="h-full tab:w-[320px] tab:shrink-0"
      >
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
              onAnimationEnd={animated ? settlePrice : undefined}
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
            // aria-disabled-ссылка остаётся фокусируемой: мышь
            // глушит pointer-events-none, Enter с клавиатуры —
            // preventDefault (паттерн ARIA disabled-link).
            <LandingLink
              variant="white"
              href={tariff.href}
              aria-disabled
              onClick={(event) => event.preventDefault()}
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
  });

  return (
    <>
      {/* Заголовок и переключатель: на планшете/мобайле — столбиком по центру,
          на десктопе — в одну строку от левого края (макет 2846-153712). */}
      <div className="flex flex-col items-center gap-6 desk:flex-row desk:items-center desk:self-stretch desk:gap-8">
        <h2 className="text-center text-[28px] font-semibold leading-8 desk:text-h2 desk:leading-[60px]">
          Тарифы
        </h2>
        {/* Переключатель периода — группа честных кнопок без панельной
            семантики (tabpanel нет), выбранное состояние — aria-pressed. */}
        <div
          className="relative flex h-14 w-64 items-center gap-0.5 rounded-[16px] bg-surface p-0.5"
          role="group"
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
              aria-pressed={billing === value}
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
      {/* Планшет: полоса во всю ширину окна — механика «Управляйте
          арендой» (TabletStrip: drag 1:1 без инерции, снап, бросок,
          Shift+Scroll — шаг; решение владельца 02.10 «как у аренды» —
          замена нативного моментума от 28.09). Ряд 3×320+gap = 984 не
          влезает в колонку секции (viewport−48) на 481–1031 — фрейм
          2859-4349 сам показывает обрез третьей карточки правым краем
          окна, то есть в макете это полоса; -mx-6 и
          w-[calc(100%+3rem)] выводят окно движка из-под контейнера
          max-w, на десктопе (≥1200) геометрия ряда прежняя. Тап по
          кнопкам карточек не перехватывается — захват указателя в
          движке отложен до первых 5px протяжки. */}
      <TabletStrip
        className="mt-8 desk:mt-14"
        nativeClassName="strip-scroll flex justify-start tab:-mx-6 tab:w-[calc(100%_+_3rem)] tab:px-6 desk:mx-0 desk:w-auto desk:px-0"
        viewportClassName="tab:-mx-6 tab:w-[calc(100%_+_3rem)]"
        activeChildren={cards}
      >
        <div className="grid w-full grid-cols-1 gap-3 tab:w-[984px] tab:shrink-0 tab:grid-cols-3 desk:w-full desk:grid-cols-3 desk:gap-5">
          {cards}
        </div>
      </TabletStrip>
    </>
  );
}
