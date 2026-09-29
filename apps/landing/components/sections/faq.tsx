"use client";

import { useState } from "react";
import { LandingLink } from "@/components/button";
import { Reveal } from "@/components/reveal";
import { FAQ } from "@/lib/content";

// «Ответы на вопросы» — макеты 2814-1065 (десктоп: две группы-карточки
// bg-surface r40, категория 36/40 слева (w-240), вопросы 18/22 справа)
// и 2859-3733 / 2826-151577 (планшет/мобайл: r32, паддинг 32/32/24,
// категория 20/24 Medium над списком с шагом 8, вопросы 16/20, шевроны
// серые 20×20, шаг между карточками 12). Внизу — CTA-карточка bg-surface
// (на планшете/мобайле по центру) — 2814-1131 / 2859-3799 / 2826-151643,
// кнопка primary «Задать вопрос» (решение владельца 29.09).
export function Faq() {
  const [open, setOpen] = useState<string | null>(null);

  return (
    <section id="faq" className="mt-24 scroll-mt-[88px] desk:mt-[156px] desk:scroll-mt-[104px]">
      <div className="mx-auto w-full max-w-[1048px] px-6 desk:max-w-[1000px] desk:px-0">
        <Reveal>
          <h2 className="text-center text-[28px] font-semibold leading-8 desk:text-h2 desk:leading-[60px]">
            Ответы на вопросы
          </h2>
        </Reveal>
        <div className="mt-8 flex flex-col gap-3 desk:mt-14 desk:gap-5">
          {FAQ.map((group, groupIndex) => (
            <Reveal key={group.category} delay={groupIndex * 100} className="w-full">
              <div className="flex flex-col gap-2 rounded-[32px] bg-surface px-8 pb-6 pt-8 desk:flex-row desk:gap-6 desk:rounded-[40px] desk:py-10 desk:pl-[52px] desk:pr-10">
                <p className="shrink-0 text-m font-medium leading-6 desk:w-[240px] desk:leading-10 desk:text-h3">
                  {group.category}
                </p>
                <div className="flex min-w-0 flex-1 flex-col">
                  {group.items.map((item) => {
                    const key = `${group.category}:${item.question}`;
                    const expanded = open === key;
                    return (
                      <div key={key} className="flex w-full flex-col py-3 tab:py-3.5 desk:py-[13px]">
                        <button
                          type="button"
                          aria-expanded={expanded}
                          onClick={() => setOpen(expanded ? null : key)}
                          className="flex w-full cursor-pointer items-center gap-2 text-left outline-none focus-visible:ring-2 focus-visible:ring-primary/40"
                        >
                          <span className="min-h-5 flex-1 text-s leading-5 text-ink desk:text-r desk:leading-[22px]">
                            {item.question}
                          </span>
                          <svg
                            width="20"
                            height="20"
                            viewBox="0 0 24 24"
                            fill="none"
                            xmlns="http://www.w3.org/2000/svg"
                            aria-hidden="true"
                            className={`shrink-0 text-gray-2 transition-transform duration-300 motion-reduce:transition-none ${
                              expanded ? "rotate-180" : ""
                            }`}
                          >
                            <path
                              d="M6 9.5L12 15.5L18 9.5"
                              stroke="currentColor"
                              strokeWidth="1.5"
                              strokeLinecap="round"
                              strokeLinejoin="round"
                            />
                          </svg>
                        </button>
                        <div
                          className={`grid transition-[grid-template-rows,opacity] duration-300 ease-out motion-reduce:transition-none ${
                            expanded
                              ? "opacity-100 [grid-template-rows:1fr]"
                              : "opacity-0 [grid-template-rows:0fr]"
                          }`}
                        >
                          <div className="overflow-hidden">
                            <p className="pt-3 text-s leading-5 text-gray-2 desk:pr-10 desk:text-r desk:leading-[22px]">
                              {item.answer}
                            </p>
                          </div>
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
            </Reveal>
          ))}
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
