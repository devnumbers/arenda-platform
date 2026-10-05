"use client";

import { useState } from "react";
import { Reveal } from "@/components/reveal";
import type { FaqGroup } from "@/lib/content";

// Раскрытый вопрос аккордеона: один на всю секцию, ключ —
// «категория:вопрос» (категории уникальны, вопросы внутри них тоже).
// Список групп приходит пропсами из серверной секции faq.tsx —
// словарь ответов не попадает в клиентский бандл.
// Геометрия (2967-76035 / 3005-78296 / 3009-79890): на мобиле/планшете
// подложка bg-surface лежит на обёртке группы (категория внутри: 22/26
// и 28/32 SemiBold, шаг до вопросов 16), на десктопе обёртка — прозрачный
// ряд (категория 36/40 Medium в колонке w-240 вне карточки), а карточкой
// (r40, px-40/py-20) становится контейнер вопросов. Вопрос 18/22 на всех
// ярусах, шеврон 24×24 #6f787c, зазор текст-иконка 16, строка py 14
// (мобайл) / 18 (планшет+). Ответ в макетах не нарисован — стили не тронуты.
export function FaqList({ groups }: { groups: FaqGroup[] }) {
  const [open, setOpen] = useState<string | null>(null);

  return (
    <>
      {groups.map((group, groupIndex) => (
        <Reveal key={group.category} delay={groupIndex * 100} className="w-full">
          <div className="flex flex-col gap-4 rounded-[32px] bg-surface px-8 pb-4 pt-8 tab:rounded-[40px] tab:px-10 tab:pb-5 tab:pt-10 desk:flex-row desk:gap-6 desk:rounded-none desk:bg-transparent desk:p-0">
            <p className="shrink-0 text-l font-semibold tab:text-h4 desk:w-[240px] desk:text-h3 desk:font-medium">
              {group.category}
            </p>
            <div className="flex min-w-0 flex-1 flex-col desk:rounded-[40px] desk:bg-surface desk:px-10 desk:py-5">
              {group.items.map((item) => {
                const key = `${group.category}:${item.question}`;
                const expanded = open === key;
                return (
                  <div key={key} className="flex w-full flex-col py-[14px] tab:py-[18px]">
                    <button
                      type="button"
                      aria-expanded={expanded}
                      onClick={() => setOpen(expanded ? null : key)}
                      className="flex w-full cursor-pointer items-center gap-4 text-left outline-none focus-visible:ring-2 focus-visible:ring-primary/40"
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
                        className={`shrink-0 text-gray-2 transition-transform duration-300 motion-reduce:transition-none ${
                          expanded ? "rotate-180" : ""
                        }`}
                      >
                        <path
                          d="M17 10L12 15L7 10"
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
    </>
  );
}
