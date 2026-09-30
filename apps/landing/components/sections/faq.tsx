import { LandingLink } from "@/components/button";
import { Reveal } from "@/components/reveal";
import { FaqList } from "@/components/sections/faq-list";
import { FAQ } from "@/lib/content";

// «Ответы на вопросы» — макеты 2814-1065 (десктоп: две группы-карточки
// bg-surface r40, категория 36/40 слева (w-240), вопросы 18/22 справа)
// и 2859-3733 / 2826-151577 (планшет/мобайл: r32, паддинг 32/32/24,
// категория 20/24 Medium над списком с шагом 8, вопросы 16/20, шевроны
// серые 20×20, шаг между карточками 12). Внизу — CTA-карточка bg-surface
// (на планшете/мобайле по центру) — 2814-1131 / 2859-3799 / 2826-151643,
// кнопка primary «Задать вопрос» (решение владельца 29.09).
// Секция серверная: словарь FAQ импортируется здесь (его же читает
// json-ld.tsx) и уходит в FaqList пропсами — клиентским остаётся только
// аккордеон.
export function Faq() {
  return (
    <section id="faq" className="mt-24 scroll-mt-[88px] desk:mt-[156px] desk:scroll-mt-[104px]">
      <div className="mx-auto w-full max-w-[1048px] px-6 desk:max-w-[1000px] desk:px-0">
        <Reveal>
          <h2 className="text-center text-[28px] font-semibold leading-8 desk:text-h2 desk:leading-[60px]">
            Ответы на вопросы
          </h2>
        </Reveal>
        <div className="mt-8 flex flex-col gap-3 desk:mt-14 desk:gap-5">
          <FaqList groups={FAQ} />
          <Reveal delay={150} className="w-full">
            {/* Хэндл Telegram-канала — плейсхолдер t.me/rentlee до финального от владельца. */}
            <div className="flex flex-col items-center gap-6 rounded-[32px] bg-surface px-8 py-16 text-center desk:flex-row desk:justify-between desk:gap-6 desk:rounded-[40px] desk:py-10 desk:pl-[52px] desk:pr-10 desk:text-left">
              <p className="text-m font-medium leading-6 desk:text-h3 desk:leading-10">
                Не нашли ответ на свой вопрос?
              </p>
              <LandingLink
                href="https://t.me/rentlee"
                target="_blank"
                rel="noopener noreferrer"
              >
                Задать вопрос
              </LandingLink>
            </div>
          </Reveal>
        </div>
      </div>
    </section>
  );
}
