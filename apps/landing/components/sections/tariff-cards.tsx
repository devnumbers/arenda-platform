"use client";

import Image from "next/image";
import { useState } from "react";
import { LandingLink } from "@/components/button";
import {
  GLASS_FILL_INK,
  GLASS_FILL_LIGHT,
  GLASS_FILL_PRIMARY,
  GLASS_GRADIENT,
  GLASS_SKIN,
} from "@/components/glass";
import { Reveal } from "@/components/reveal";
import { TabletStrip } from "@/components/tablet-strip";
import { useSwapPhase } from "@/components/use-swap-phase";
import iconHomeMain from "@/assets/icons/icon-home-main.svg";
import iconObjects from "@/assets/icons/icon-objects.svg";
import iconTeam from "@/assets/icons/icon-team.svg";
import iconTimeHistory from "@/assets/icons/icon-time-history.svg";
import type { Tariff, TariffFeature } from "@/lib/content";

// «Тарифы» — макеты 3448-78571 (десктоп: 3×320×597, r40, p-40: шапка
// (название + цена), кнопка CTA сразу под ней, фичи ниже — кадр перенёс
// кнопку из подвала карточки вверх; «Про» и «Бизнес» получили третью
// фичу «История действий» с Icon/R/TimeHistory; фиксированная высота
// только на десктопе, на мобиле/планшете карточки по контенту и тянутся
// в ряд по самой высокой). Шкурки по кадрам
// карточек 2967:75983/75995/76015: «Базовый» — плоская #F3F4F6,
// обводка/стекло/блики/блюр сняты; «Про» и «Бизнес» остаются
// стеклянными (коническое кольцо .glass-ring, стеклянный градиент,
// белые inset-блики, blur 10), градиенты заливок новые: тёмный
// 50,50,50→30,30,30, синий 63,147,255→43,127,255. Константы стекла —
// общие с «Для кого сервис» (components/glass.ts), заголовки фич —
// Landing/M (Medium 500). Цена — 32/36 SemiBold на всех ярусах
// (новые кадры тарифных карточек). Переключатель 256×56: активный
// сегмент — белая пилюля, переезжающая между Год/Месяц за 300мс
// (iOS-сегментконтрол); бейдж «−25%» живёт в сегменте «Год» всегда.
// Цена при переключении меняется в хореографии steps (180мс уход →
// 280мс въезд, токен --animate-tariff-price-in). Кнопка «Ваш тариф» —
// White Disabled
// (серый текст). CTA платных карточек ведёт на экран «Выбрать тариф»
// кабинета с tariff и текущим периодом в query — экран предвыбирает
// их из URL (гость доводится логином через from= прокси кабинета).
const PRICE_OUT_MS = 180;

const FEATURE_ICONS: Record<TariffFeature["icon"], { src: string; alt: string }> = {
  home: { src: iconHomeMain.src, alt: "Иконка объекта" },
  objects: { src: iconObjects.src, alt: "Иконка объектов" },
  team: { src: iconTeam.src, alt: "Иконка команды" },
  history: { src: iconTimeHistory.src, alt: "Иконка истории действий" },
};

const CARD_THEMES = {
  basic: {
    backgroundImage: undefined,
    backgroundColor: GLASS_FILL_LIGHT,
    glass: false,
    text: "text-ink",
    sub: "text-gray-2",
    featureSub: "text-gray-2",
  },
  pro: {
    backgroundImage: `${GLASS_GRADIENT}, ${GLASS_FILL_INK}`,
    backgroundColor: "transparent",
    glass: true,
    text: "text-white",
    sub: "text-white/80",
    featureSub: "text-white/80",
  },
  business: {
    backgroundImage: `${GLASS_GRADIENT}, ${GLASS_FILL_PRIMARY}`,
    backgroundColor: "transparent",
    glass: true,
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
    // CTA платной карточки ведёт на «Выбрать тариф» с предвыбором этого
    // тарифа и текущего периода переключателя (словарь лендинга yearly/
    // monthly переводится в словарь кабинета year/month, который читает
    // tariffChangeUrlOverrides); у базового — /login.
    const ctaHref =
      tariff.id === "basic"
        ? tariff.href
        : `${tariff.href}?tariff=${tariff.id}&period=${
            billing === "yearly" ? "year" : "month"
          }`;
    return (
      <Reveal
        key={tariff.id}
        delay={index * 80}
        /* self-stretch, не h-full: в авторазмерном ряду (планшетная
           полоса, сетки мобайл/десктоп) процентная высота против
           неопределённой строки даёт auto — «Базовый» оставался ниже
           платных карточек; stretch вытягивает обёртку по самой
           высокой, article h-full добивает уже от неё. */
        className="self-stretch tab:w-[320px] tab:shrink-0"
      >
        <article
          style={{
            backgroundImage: theme.backgroundImage,
            backgroundColor: theme.backgroundColor,
          }}
          className={`${theme.glass ? GLASS_SKIN : ""} relative flex h-full w-full flex-col gap-8 overflow-clip rounded-[32px] p-8 tab:w-[320px] desk:h-[597px] desk:rounded-[40px] desk:p-10 ${theme.text}`}
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
                  ? "-translate-y-1 opacity-0 transition-[opacity,translate] duration-[180ms] ease-inout"
                  : animated && pricePhase === "entering"
                    ? "animate-tariff-price-in"
                    : ""
              }`}
            >
              <p className="text-[32px] font-semibold leading-9">{price}</p>
              {/* Строка года резервируется (invisible) и у «Базового», и в
                  «Месяце»: кнопка CTA стоит сразу под шапкой (кадр
                  3448-78571) — без резерва кнопки карточек разъезжались бы
                  по высоте, у «Базового» на 20px выше платных. */}
              <p
                className={`text-s leading-5 desk:text-r desk:leading-[22px] ${theme.sub} ${
                  tariff.id !== "basic" && displayedBilling === "yearly" ? "" : "invisible"
                }`}
              >
                {tariff.id !== "basic"
                  ? `При оплате ${tariff.yearlyTotal.toLocaleString("ru-RU")} ₽ за год`
                  : /* Резерв высоты у бесплатной карточки — пустой <p>
                       высоты не имеет, строка живёт неразрывным пробелом. */
                    "\u00A0"}
              </p>
            </div>
          </div>
          {isCurrent ? (
            // aria-disabled-ссылка остаётся фокусируемой: мышь
            // глушит pointer-events-none, Enter с клавиатуры —
            // preventDefault (паттерн ARIA disabled-link).
            <LandingLink
              variant="white"
              href={ctaHref}
              aria-disabled
              onClick={(event) => event.preventDefault()}
              className="pointer-events-none w-full text-gray-2!"
            >
              Ваш тариф
            </LandingLink>
          ) : (
            <LandingLink variant="white" href={ctaHref} className="w-full">
              {tariff.cta}
            </LandingLink>
          )}
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
        </article>
      </Reveal>
    );
  });

  return (
    <>
      {/* Заголовок и переключатель: на планшете/мобайле — столбиком по центру,
          на десктопе — в одну строку от левого края (макет 2846-153712). */}
      <div className="flex flex-col items-center gap-6 desk:flex-row desk:items-center desk:self-stretch desk:gap-8">
        <h2 className="text-center text-[28px] font-semibold leading-8 tab:text-[32px] tab:leading-9 desk:text-h2 desk:leading-[60px]">
          Тарифы
        </h2>
        {/* Переключатель периода — группа честных кнопок без панельной
            семантики (tabpanel нет), выбранное состояние — aria-pressed.
            overflow-clip = «Clip content» кадра Figma 2967-75975: тень
            пилюли не выходит за капсулу (клип чтёт радиус 16px). */}
        <div
          className="relative flex h-14 w-64 items-center gap-0.5 overflow-clip rounded-[16px] bg-surface p-0.5"
          role="group"
          aria-label="Период оплаты"
        >
          <span
            aria-hidden
            className="absolute inset-y-0.5 left-0.5 w-[125px] rounded-[14px] bg-white shadow-[0_2px_8px_rgba(0,0,0,0.16)] transition-transform duration-300 ease-inout"
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
