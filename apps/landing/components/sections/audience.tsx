import Image from "next/image";
import {
  GLASS_FILL_INK,
  GLASS_FILL_LIGHT,
  GLASS_FILL_PRIMARY,
  GLASS_GRADIENT,
  GLASS_INSET_SHADOW,
} from "@/components/glass";
import { Reveal } from "@/components/reveal";
import { TabletStrip } from "@/components/tablet-strip";
import iconBriefcase01 from "@/assets/icons/icon-briefcase-01.svg";
import iconBuilding05 from "@/assets/icons/icon-building-05.svg";
import iconUser02 from "@/assets/icons/icon-user-02.svg";

// «Для кого сервис» — макеты 2851-154065 (десктоп: три стеклянные
// карточки 320×320, r40, p-40, шаг 64 после заголовка), 2851-154111
// (мобайл: стопка 345×280, r32, p-32, шаг 32); иконка 64×64, заголовок
// карточки 28/32 Medium и текст 18/22, text-balance на заголовке секции
// и текстах карточек (свойство стоит в узлах Figma) — на всех
// брейкпоинтах.
// Шкурка плашек — та же, что у тарифов: карточки-спеки 2851-154068
// (154069/154075/154081) дизайнер обновил под тарифные — обводка теперь
// конический градиент .glass-ring (тот же токен, что у 2846-153711;
// прежнее плоское серое inset-кольцо rgba(156,156,156,0.5) снято),
// заливки — на самой карточке, у «Собственников» сплошной #F3F4F6
// под стеклом, как у «Базового». Стеклянный градиент, блики и заливки —
// общие константы components/glass.ts.
const CARDS = [
  {
    title: "Собственники",
    text: "Сдаете квартиру, дом, комнату, гараж или помещение",
    icon: iconUser02,
    alt: "Иконка собственника",
    backgroundImage: GLASS_GRADIENT,
    backgroundColor: GLASS_FILL_LIGHT,
    light: true,
  },
  {
    title: "Бизнес",
    text: "Сдаете офисы, склады, торговые помещения, гаражи",
    icon: iconBriefcase01,
    alt: "Иконка бизнеса",
    backgroundImage: `${GLASS_GRADIENT}, ${GLASS_FILL_INK}`,
    backgroundColor: "transparent",
    light: false,
  },
  {
    title: "Агентства",
    text: "Сдаете десятки или сотни объектов",
    icon: iconBuilding05,
    alt: "Иконка агентства",
    backgroundImage: `${GLASS_GRADIENT}, ${GLASS_FILL_PRIMARY}`,
    backgroundColor: "transparent",
    light: false,
  },
];

export function Audience() {
  return (
    <section id="audience" className="mt-24 scroll-mt-[88px] desk:mt-[156px] desk:scroll-mt-[104px]">
      <div className="mx-auto w-full max-w-[1048px] px-6 desk:max-w-[1000px] desk:px-0">
        <Reveal>
          <h2 className="text-center text-[28px] font-semibold leading-8 text-balance desk:text-h2 desk:leading-[60px]">
            Для кого сервис
          </h2>
        </Reveal>
      </div>
      {/* Планшет: полоса во всю ширину окна — поведение CardTrio
          («Организуйте дела»), движок «Управляйте арендой» (TabletStrip:
          drag 1:1 без инерции, снап, бросок, Shift+Scroll — шаг;
          решение владельца 02.10 «как у аренды» — замена нативного
          моментума от 28.09). Когда три карточки влезают (1032–1199),
          полоса центрируется — на стыке с десктопным рядом скачка
          геометрии нет. */}
      <TabletStrip
        className="mt-8 w-full desk:mt-16"
        nativeClassName="strip-scroll flex flex-col gap-3 px-6 tab:mx-auto tab:w-fit tab:max-w-full tab:flex-row desk:gap-5 desk:px-0"
        centerWhenFit
      >
        {CARDS.map((card, index) => (
          <Reveal
            key={card.title}
            delay={index * 100}
            className="h-full tab:w-[320px] tab:shrink-0"
          >
            <article
              style={{
                backgroundImage: card.backgroundImage,
                backgroundColor: card.backgroundColor,
              }}
              className={`glass-ring relative flex h-[280px] w-full flex-col justify-between overflow-clip rounded-[32px] p-8 backdrop-blur-[10px] tab:h-[320px] desk:aspect-square desk:h-auto desk:rounded-[40px] desk:p-10 ${GLASS_INSET_SHADOW}`}
            >
              <Image
                src={card.icon}
                alt={card.alt}
                width={64}
                height={64}
                className="size-16"
              />
              <div
                className={`flex flex-col gap-3 ${
                  card.light ? "text-ink" : "text-white"
                }`}
              >
                <h3 className="text-[28px] font-medium leading-8 text-balance">
                  {card.title}
                </h3>
                <p
                  className={`text-r text-balance ${
                    card.light ? "text-gray-2" : "text-white"
                  }`}
                >
                  {card.text}
                </p>
              </div>
            </article>
          </Reveal>
        ))}
      </TabletStrip>
    </section>
  );
}
