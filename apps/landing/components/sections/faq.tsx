"use client";

import { useState } from "react";
import { LandingLink } from "@/components/button";
import { Reveal } from "@/components/reveal";
import { FAQ } from "@/lib/content";

// «Ответы на вопросы» — макет 2814-1065: H2 56, две группы-карточки
// (bg-surface radius-40, pl-52 pr-40 py-40): категория 36/40 Medium слева
// (w-240), вопросы справа (строки py-12: вопрос 18/22 + шеврон 24×24,
// gap-16). Ответ раскрывается плавно (grid-rows), шеврон поворачивается.
// Внизу — «Не нашли ответ на свой вопрос?» + кнопка Telegram.
export function Faq() {
  const [open, setOpen] = useState<string | null>(null);

  return (
    <section id="faq" className="mt-20 desk:mt-[156px]">
      <div className="mx-auto w-full max-w-[1000px] px-5 desk:px-10">
        <Reveal>
          <h2 className="text-center text-[28px] font-semibold leading-8 desk:text-h2 desk:leading-[60px]">
            Ответы на вопросы
          </h2>
        </Reveal>
        <div className="mt-14 flex flex-col gap-5">
          {FAQ.map((group, groupIndex) => (
            <Reveal key={group.category} delay={groupIndex * 100} className="w-full">
              <div className="flex flex-col gap-6 rounded-[40px] bg-surface py-10 pl-8 pr-8 desk:flex-row desk:gap-6 desk:pl-[52px] desk:pr-10">
                <p className="shrink-0 text-xl font-medium leading-[40px] desk:w-[240px] desk:text-h3">
                  {group.category}
                </p>
                <div className="flex min-w-0 flex-1 flex-col">
                  {group.items.map((item) => {
                    const key = `${group.category}:${item.question}`;
                    const expanded = open === key;
                    return (
                      <div key={key} className="flex w-full flex-col py-3">
                        <button
                          type="button"
                          aria-expanded={expanded}
                          onClick={() => setOpen(expanded ? null : key)}
                          className="flex w-full items-center gap-4 text-left outline-none focus-visible:ring-2 focus-visible:ring-primary/40"
                        >
                          <span className="min-h-6 flex-1 text-r text-ink">
                            {item.question}
                          </span>
                          <svg
                            width="24"
                            height="24"
                            viewBox="0 0 24 24"
                            fill="none"
                            xmlns="http://www.w3.org/2000/svg"
                            aria-hidden="true"
                            className={`shrink-0 transition-transform duration-300 ${
                              expanded ? "rotate-180" : ""
                            }`}
                          >
                            <path
                              d="M6 9.5L12 15.5L18 9.5"
                              stroke="#171A1C"
                              strokeWidth="1.5"
                              strokeLinecap="round"
                              strokeLinejoin="round"
                            />
                          </svg>
                        </button>
                        <div
                          className={`grid transition-[grid-template-rows,opacity] duration-300 ease-out ${
                            expanded
                              ? "opacity-100 [grid-template-rows:1fr]"
                              : "opacity-0 [grid-template-rows:0fr]"
                          }`}
                        >
                          <div className="overflow-hidden">
                            <p className="pr-10 pt-3 text-r text-gray-2">
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
        </div>
        <Reveal delay={150} className="w-full">
          <div className="mt-10 flex flex-col items-start justify-between gap-6 pl-8 pr-8 desk:flex-row desk:items-center desk:pl-[52px]">
            <p className="text-xl font-medium leading-6 desk:text-h3 desk:leading-10">
              Не нашли ответ на свой вопрос?
            </p>
            {/* TODO(владелец): хэндл Telegram-канала — плейсхолдер t.me/rentlee */}
            <LandingLink
              variant="gray"
              href="https://t.me/rentlee"
              target="_blank"
              rel="noopener noreferrer"
            >
              Написать в Telegram
            </LandingLink>
          </div>
        </Reveal>
      </div>
    </section>
  );
}
