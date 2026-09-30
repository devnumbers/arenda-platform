"use client";

import { useState } from "react";
import { Reveal } from "@/components/reveal";
import type { FaqGroup } from "@/lib/content";

// Раскрытый вопрос аккордеона: один на всю секцию, ключ —
// «категория:вопрос» (категории уникальны, вопросы внутри них тоже).
// Список групп приходит пропсами из серверной секции faq.tsx —
// словарь ответов не попадает в клиентский бандл.
export function FaqList({ groups }: { groups: FaqGroup[] }) {
  const [open, setOpen] = useState<string | null>(null);

  return (
    <>
      {groups.map((group, groupIndex) => (
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
    </>
  );
}
