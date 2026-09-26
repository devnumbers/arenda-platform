"use client";

import Image from "next/image";
import { useState } from "react";
import { LandingLink } from "@/components/button";
import { Reveal } from "@/components/reveal";
import { STEPS } from "@/lib/content";

// «Как начать пользоваться» — макет 2859-2531 (+ состояния 2859-2455/2493/
// 2512/2474): H2 56, 4 круглых StepButton 44×44 (активная синяя с белой
// цифрой 24 Bold, остальные #2b7fff1a с #95bfff), текст шага 22/26
// opacity-50, мокап телефона 273×400, синяя кнопка «Начать пользоваться».
export function Steps() {
  const [active, setActive] = useState(0);
  const step = STEPS[active] ?? STEPS[0];
  if (!step) {
    return null;
  }

  return (
    <section id="steps" className="mt-20 desk:mt-[156px]">
      <div className="mx-auto flex w-full max-w-[1000px] flex-col items-center px-5 desk:px-10">
        <Reveal className="w-full">
          <div className="flex w-full flex-col items-center gap-12">
            <h2 className="text-center text-[28px] font-semibold leading-8 desk:text-h2 desk:leading-[60px]">
              Как начать пользоваться
            </h2>
            <div className="flex items-center gap-4" role="tablist" aria-label="Шаги">
              {STEPS.map((item, index) => (
                <button
                  key={item.text}
                  type="button"
                  role="tab"
                  aria-selected={active === index}
                  aria-label={`Шаг ${index + 1}`}
                  onClick={() => setActive(index)}
                  className={`flex size-11 items-center justify-center rounded-[100px] text-xl font-bold transition-colors duration-200 outline-none focus-visible:ring-2 focus-visible:ring-primary/40 ${
                    active === index
                      ? "bg-primary text-white"
                      : "bg-primary-light text-primary-disabled [text-shadow:0_8px_24px_rgba(43,127,255,0.08)]"
                  }`}
                >
                  {index + 1}
                </button>
              ))}
            </div>
            <p className="text-center text-l leading-[26px] text-ink opacity-50">
              {step.text}
            </p>
          </div>
        </Reveal>
        <Reveal delay={100}>
          <div className="relative mt-14 h-[400px] w-[273px]">
            {STEPS.map((item, index) => (
              <Image
                key={item.text}
                src={item.image}
                alt={item.alt}
                width={273}
                height={400}
                sizes="273px"
                priority={false}
                className={`absolute inset-0 transition-opacity duration-300 ${
                  active === index ? "opacity-100" : "opacity-0"
                }`}
              />
            ))}
          </div>
        </Reveal>
        <Reveal delay={150}>
          <LandingLink href="/login" className="mt-14">
            Начать пользоваться
          </LandingLink>
        </Reveal>
      </div>
    </section>
  );
}
