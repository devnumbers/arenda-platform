import Image, { type StaticImageData } from "next/image";
import { Reveal } from "@/components/reveal";
import { TabletStrip } from "@/components/tablet-strip";

// Общая секция-трио карточек с UI-моками (макеты 2814-923 «финансы»,
// 2814-937 «организуйте дела», 2859-4194, 2826-151417): десктоп — ряд
// 3×320×550 (r40, p-40, контент прижат к низу); планшет (481–1199) — та
// же карточка 320×500 (r32, p-32) в полосе на движке «Управляйте
// арендой» (TabletStrip: drag 1:1 без инерции, снап, бросок,
// Shift+Scroll — шаг; решение владельца 02.10 «как у аренды» — замена
// нативного моментума от 28.09); мобайл — стопка по ширине колонки,
// высота по контенту (r32, p-32, шаг текст→моки 48); заголовок карточки
// 22/26, текст 16/20 (десктоп 28/32 и 18/22); text-balance на заголовке
// секции и текстах карточек — свойство стоит в узлах Figma
// (2814-923/937).
export type CardTrioCard = {
  title: string;
  text: string;
  images: { src: StaticImageData; alt: string }[];
};

export function CardTrio({
  id,
  title,
  cards,
}: {
  id: string;
  title: string;
  cards: CardTrioCard[];
}) {
  return (
    <section id={id} className="mt-24 scroll-mt-[88px] desk:mt-[156px] desk:scroll-mt-[104px]">
      <div className="mx-auto w-full max-w-[1048px] px-6 desk:max-w-[1000px] desk:px-0">
        <Reveal>
          <h2 className="text-center text-[28px] font-semibold leading-8 text-balance desk:text-h2 desk:leading-[60px]">
            {title}
          </h2>
        </Reveal>
      </div>
      {/* Планшет: полоса стартует от поля 24px и течёт за правый край
          окна; когда три карточки влезают (1032–1199), центрируется —
          как нативная w-fit + mx-auto до замены механики. */}
      <TabletStrip
        className="mt-8 w-full desk:mt-14"
        nativeClassName="flex flex-col gap-3 px-6 tab:mx-auto tab:w-fit tab:max-w-full tab:flex-row tab:hide-scrollbar desk:gap-5 desk:px-0"
        centerWhenFit
      >
        {cards.map((card, index) => (
          <Reveal
            key={card.title}
            delay={index * 100}
            className="h-full tab:w-[320px] tab:shrink-0"
          >
            <article className="flex h-auto w-full flex-col gap-12 overflow-clip rounded-[32px] bg-surface p-8 tab:h-[500px] tab:justify-between desk:h-[550px] desk:rounded-[40px] desk:p-10">
              <div className="flex flex-col gap-2 desk:gap-3">
                <h3 className="text-[22px] font-medium leading-[26px] text-balance desk:text-[28px] desk:leading-8">
                  {card.title}
                </h3>
                <p className="text-s leading-5 text-gray-2 text-balance desk:text-r desk:leading-[22px]">
                  {card.text}
                </p>
              </div>
              <div className="flex flex-col gap-2 desk:gap-4">
                {card.images.map((image) => (
                  <Image
                    key={image.alt}
                    src={image.src}
                    alt={image.alt}
                    sizes="(min-width: 481px) 320px, 100vw"
                    className="h-auto w-full rounded-[20px] shadow-[0_4px_16px_rgba(0,0,0,0.04)]"
                  />
                ))}
              </div>
            </article>
          </Reveal>
        ))}
      </TabletStrip>
    </section>
  );
}
