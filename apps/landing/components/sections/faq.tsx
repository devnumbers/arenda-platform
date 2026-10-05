import { Reveal } from "@/components/reveal";
import { FaqList } from "@/components/sections/faq-list";
import { FeedbackModal } from "@/components/feedback-modal";
import { FAQ } from "@/lib/content";

// «Ответы на вопросы» — макеты 2967-76035 (десктоп: категория 36/40 Medium
// в колонке w-240 СЛЕВА от карточки — сама карточка bg-surface r40 px-40/
// py-20 держит только список вопросов; CTA прижат влево к колонке карточек,
// отступ 240+24, зазор от карточки 56), 3005-78296 (планшет: тайтл 32/36,
// карточка r40 40/40/20, категория 28/32 SemiBold внутри, шаг до вопросов 16)
// и 3009-79890 (мобайл: карточка r32 32/32/16, категория 22/26 SemiBold —
// аннотация дизайнера «Другой шрифт 22px semibold 26px line height»).
// Вопросы 18/22 и шеврон 24×24 на всех ярусах (зазор текст-иконка 16);
// border-b строк в макете совпадает цветом с карточкой — не рисуем.
// Внизу CTA без карточки-подложки: текст 28/32 Regular (стиль макета
// «H4 Regular», решение владельца) + кнопка primary «Задать вопрос» —
// планшет/мобайл по центру в 96 от карточки, текст-кнопка 32; хэндл
// Telegram-канала — плейсхолдер t.me/rentlee до финального от владельца.
// Ответ аккордеона в макетах не нарисован — намеренно не тронут (решение
// владельца: правки строго по макету).
// Секция серверная: словарь FAQ импортируется здесь (его же читает
// json-ld.tsx) и уходит в FaqList пропсами — клиентским остаётся только
// аккордеон.
export function Faq() {
  return (
    <section id="faq" className="mt-24 scroll-mt-[88px] desk:mt-[156px] desk:scroll-mt-[104px]">
      <div className="mx-auto w-full max-w-[1048px] px-6 desk:max-w-[1000px] desk:px-0">
        <Reveal>
          <h2 className="text-center text-[28px] font-semibold leading-8 tab:text-[32px] tab:leading-9 desk:text-h2 desk:leading-[60px]">
            Ответы на вопросы
          </h2>
        </Reveal>
        <div className="mt-8 flex flex-col gap-3 desk:mt-14 desk:gap-5">
          <FaqList groups={FAQ} />
        </div>
        <Reveal delay={150} className="w-full">
          <div className="mt-24 flex flex-col items-center gap-8 text-center desk:mt-14 desk:ml-[264px] desk:items-start desk:text-left">
            <p className="text-h4 text-balance">Не нашли ответ на свой вопрос?</p>
            {/* Кнопка открывает модалку «Задать вопрос» (форма уходит письмом
                через /api/feedback → бекенд); размер sm — инстанс LandingButton
                177×56 из кадра 2967-76035. */}
            <FeedbackModal label="Задать вопрос" size="sm" />
          </div>
        </Reveal>
      </div>
    </section>
  );
}
