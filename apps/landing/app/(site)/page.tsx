// Плейсхолдеры секций фундамента (тикет T1 карты #888): каркас страницы
// в порядке макета 2814-728. Секции приходят тикетами T3–T5 — каждая
// заменяет свою заглушку компонентом из components/sections/.
// Хедер, футер и маркер id="landing-root" приходят из (site)/layout.tsx.

import { Audience } from "@/components/sections/audience";
import { Finance } from "@/components/sections/finance";
import { Hero } from "@/components/sections/hero";
import { Organization } from "@/components/sections/organization";
import { Rentals } from "@/components/sections/rentals";
import { Sharing } from "@/components/sections/sharing";
import { Showcase } from "@/components/sections/showcase";
import { Testimonials } from "@/components/sections/testimonials";

type SectionStub = { id: string; title: string };

const SECTIONS: SectionStub[] = [
  { id: "steps", title: "Как начать пользоваться" },
  { id: "tariffs", title: "Тарифы" },
  { id: "faq", title: "Ответы на вопросы" },
  { id: "contact", title: "Остались вопросы?" },
  { id: "cta", title: "Попробуйте Рентли в деле" },
];

export default function LandingPage() {
  return (
    <>
      <Hero />
      <Showcase />
      <Rentals />
      <Finance />
      <Organization />
      <Sharing />
      <Audience />
      <Testimonials />
      {SECTIONS.map((section) => (
        <section
          key={section.id}
          id={section.id}
          className="mx-auto mt-20 w-full max-w-[1000px] px-5 py-16 desk:mt-[156px] md:px-10 lg:px-0"
        >
          <h2 className="text-h4 font-semibold md:text-h3">{section.title}</h2>
          <p className="mt-3 text-r text-gray-2">
            Секция верстается тикетом карты #888.
          </p>
        </section>
      ))}
    </>
  );
}
