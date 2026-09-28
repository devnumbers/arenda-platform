import Image, { type StaticImageData } from "next/image";
import { Reveal } from "@/components/reveal";

// Общая секция-трио карточек с UI-моками (макеты 2814-923 «финансы»,
// 2814-937 «организуйте дела», 2859-4194, 2826-151417): десктоп — ряд
// 3×320×550 (r40, p-40, контент прижат к низу); планшет (481–1199) — та же
// карточка 320×500 (r32, p-32) в горизонтальной скролл-полосе во всю ширину
// окна: нативный тач-моментум, скрытый скроллбар, overscroll-x-contain,
// отступ 24px до первой и после последней — как у карусели аренды, но без
// снапа, докатки и точек (решение владельца 28.09: свайп замирает там, где
// его отпустили, — CSS-снап не добавляется, JS не нужен); мобайл — стопка
// по ширине колонки, высота по контенту (r32, p-32, шаг текст→моки 48);
// заголовок карточки 22/26, текст 16/20 (десктоп 28/32 и 18/22).
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
    <section id={id} className="mt-24 desk:mt-[156px]">
      <div className="mx-auto w-full max-w-[1048px] px-6 desk:max-w-[1000px] desk:px-0">
        <Reveal>
          <h2 className="text-center text-[28px] font-semibold leading-8 desk:text-h2 desk:leading-[60px]">
            {title}
          </h2>
        </Reveal>
      </div>
      {/* Планшет: полоса во всю ширину окна; когда три карточки влезают
          (1032–1199), max-w-full сжимает её до контента (голый w-fit не
          сжимается: у nowrap-ряда min-content равен полной ширине) и
          mx-auto ставит по центру — на стыке с десктопным рядом (1000 по
          центру) скачка геометрии нет. */}
      <div className="mt-8 flex w-full flex-col gap-3 px-6 tab:mx-auto tab:w-fit tab:max-w-full tab:flex-row tab:overflow-x-auto tab:overscroll-x-contain [scrollbar-width:none] [&::-webkit-scrollbar]:hidden desk:mt-14 desk:gap-5 desk:px-0">
        {cards.map((card, index) => (
          <Reveal
            key={card.title}
            delay={index * 100}
            className="h-full tab:w-[320px] tab:shrink-0"
          >
            <article className="flex h-auto w-full flex-col gap-12 overflow-clip rounded-[32px] bg-surface p-8 tab:h-[500px] tab:justify-between desk:h-[550px] desk:rounded-[40px] desk:p-10">
              <div className="flex flex-col gap-2 desk:gap-3">
                <h3 className="text-[22px] font-medium leading-[26px] desk:text-[28px] desk:leading-8">
                  {card.title}
                </h3>
                <p className="text-s leading-5 text-gray-2 desk:text-r desk:leading-[22px]">
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
      </div>
    </section>
  );
}
