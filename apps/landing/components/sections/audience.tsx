import Image from "next/image";
import { Reveal } from "@/components/reveal";
import iconBriefcase01 from "@/assets/icons/icon-briefcase-01.svg";
import iconBuilding05 from "@/assets/icons/icon-building-05.svg";
import iconUser02 from "@/assets/icons/icon-user-02.svg";

// «Для кого сервис» — макеты 2851-154065 (десктоп: три стеклянные
// карточки 320×320, r40, p-40, шаг 64 после заголовка), 2859-4242
// (планшет: ряд 3×320×320 r32 с вылезанием за страницу), 2851-154111
// (мобайл: стопка 345×280, r32, p-32, шаг 32); иконка 64×64, заголовок
// карточки 28/32 Medium и текст 18/22 — на всех брейкпоинтах.
const GLASS_INSET_SHADOW =
  "shadow-[inset_0px_-5px_4px_0px_rgba(255,255,255,0.25),inset_0px_4px_4px_0px_rgba(255,255,255,0.25)]";

const CARDS = [
  {
    title: "Собственники",
    text: "Сдаете квартиру, дом, комнату, гараж или помещение",
    icon: iconUser02,
    alt: "Иконка собственника",
    gradient:
      "linear-gradient(180deg, rgba(255,255,255,0.1) 0%, rgba(217,217,217,0.1) 100%), linear-gradient(90deg, rgb(243,244,246) 0%, rgb(243,244,246) 100%)",
    light: true,
  },
  {
    title: "Бизнес",
    text: "Сдаете офисы, склады, торговые помещения, гаражи",
    icon: iconBriefcase01,
    alt: "Иконка бизнеса",
    gradient:
      "linear-gradient(180deg, rgba(255,255,255,0.1) 0%, rgba(217,217,217,0.1) 100%), linear-gradient(180deg, rgb(100,100,100) 0%, rgb(30,30,30) 100%)",
    light: false,
  },
  {
    title: "Агентства",
    text: "Сдаете десятки или сотни объектов",
    icon: iconBuilding05,
    alt: "Иконка агентства",
    gradient:
      "linear-gradient(180deg, rgba(255,255,255,0.1) 0%, rgba(217,217,217,0.1) 100%), linear-gradient(180deg, rgb(136,183,255) 0%, rgb(43,127,255) 100%)",
    light: false,
  },
];

export function Audience() {
  return (
    <section id="audience" className="mt-24 desk:mt-[156px]">
      <div className="mx-auto w-full max-w-[1048px] px-6 desk:max-w-[1000px] desk:px-0">
        <Reveal>
          <h2 className="text-center text-[28px] font-semibold leading-8 desk:text-h2 desk:leading-[60px]">
            Для кого сервис
          </h2>
        </Reveal>
        <div className="mt-8 flex justify-center desk:mt-16">
          <div className="grid w-full grid-cols-1 gap-3 tab:w-[984px] tab:shrink-0 tab:grid-cols-3 desk:w-full desk:grid-cols-3 desk:gap-5">
            {CARDS.map((card, index) => (
              <Reveal key={card.title} delay={index * 100}>
                <article
                  className={`relative flex h-[280px] flex-col justify-between overflow-clip rounded-[32px] border border-[rgba(156,156,156,0.5)] p-8 backdrop-blur-[10px] tab:h-[320px] desk:aspect-square desk:h-auto desk:rounded-[40px] desk:p-10 ${GLASS_INSET_SHADOW}`}
                >
                  <div
                    aria-hidden
                    className="pointer-events-none absolute inset-0 rounded-[40px]"
                    style={{ backgroundImage: card.gradient }}
                  />
                  <Image
                    src={card.icon}
                    alt={card.alt}
                    width={64}
                    height={64}
                    className="relative size-16"
                  />
                  <div
                    className={`relative flex flex-col gap-3 ${
                      card.light ? "text-ink" : "text-white"
                    }`}
                  >
                    <h3 className="text-[28px] font-medium leading-8">
                      {card.title}
                    </h3>
                    <p
                      className={`text-r ${
                        card.light ? "text-gray-2" : "text-white"
                      }`}
                    >
                      {card.text}
                    </p>
                  </div>
                </article>
              </Reveal>
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}
