import Image, { type StaticImageData } from "next/image";
import { Reveal } from "@/components/reveal";

// Общая секция-трио карточек с UI-моками (макеты 2814-923 «финансы» и
// 2814-937 «организуйте дела»): H2 по центру + 3 карточки 320×550
// (radius-40, #f3f4f6, p-40, контент прижат к низу), сверху заголовок
// 28 Medium + текст 18/22 gray-2 (шаг 12), снизу моки (radius-20, тень
// 0 4 16 .04).
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
    <section id={id} className="mt-20 desk:mt-[156px]">
      <div className="mx-auto w-full max-w-[1000px] px-5 desk:px-10">
        <Reveal>
          <h2 className="text-center text-[28px] font-semibold leading-8 desk:text-h2 desk:leading-[60px]">
            {title}
          </h2>
        </Reveal>
        <div className="mt-14 grid gap-5 desk:grid-cols-3">
          {cards.map((card, index) => (
            <Reveal key={card.title} delay={index * 100} className="h-full">
              <article className="flex h-[550px] flex-col justify-between overflow-clip rounded-[40px] bg-surface p-10">
                <div className="flex flex-col gap-3">
                  <h3 className="text-m font-medium leading-6 desk:text-[28px] desk:leading-8">
                    {card.title}
                  </h3>
                  <p className="text-r text-gray-2">{card.text}</p>
                </div>
                <div className="flex flex-col gap-4">
                  {card.images.map((image) => (
                    <Image
                      key={image.alt}
                      src={image.src}
                      alt={image.alt}
                      sizes="(min-width: 1200px) 320px, (min-width: 481px) 33vw, 100vw"
                      className="h-auto w-full rounded-[20px] shadow-[0_4px_16px_rgba(0,0,0,0.04)]"
                    />
                  ))}
                </div>
              </article>
            </Reveal>
          ))}
        </div>
      </div>
    </section>
  );
}
