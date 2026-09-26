"use client";

import Image from "next/image";
import { useState } from "react";
import { LandingLink } from "@/components/button";
import { Reveal } from "@/components/reveal";
import { STEPS } from "@/lib/content";

// «Как начать пользоваться» — макеты 2859-2531 (десктоп), 2859-3647
// (планшет), 2826-152026 (мобайл): градиентная подложка (прозрачный →
// синий 10%, r32, контент прижат к низу — на десктопе с боковыми
// полями 8, на планшете/мобайле во всю ширину страницы), H2 + 4 круглых
// StepButton 44×44 + текст шага (десктоп 22/26, планшет/мобайл 16/20,
// opacity-50), мокап телефона 273×400; синяя кнопка — ПОД подложкой
// (шаг 32, десктоп 56).
export function Steps() {
  const [active, setActive] = useState(0);
  const step = STEPS[active] ?? STEPS[0];
  if (!step) {
    return null;
  }

  return (
    <section id="steps" className="mt-24 desk:mt-[156px]">
      <div className="flex flex-col items-center gap-8 desk:gap-14">
        <div className="w-full px-0 desk:px-2">
          <div className="relative flex w-full flex-col items-center justify-end overflow-hidden rounded-[32px] bg-[linear-gradient(180deg,rgba(43,127,255,0)_0%,rgba(43,127,255,0.1)_100%)] px-8 desk:px-10">
            <Reveal className="w-full">
              <div className="flex w-full flex-col items-center gap-6 desk:gap-8">
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
                <p className="mx-auto max-w-[250px] text-center text-s leading-5 text-ink opacity-50 tab:max-w-none desk:text-l desk:leading-[26px]">
                  {step.text}
                </p>
              </div>
            </Reveal>
            <Reveal delay={100} className="mt-14">
              <div className="relative h-[400px] w-[273px]">
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
          </div>
        </div>
        <Reveal delay={150}>
          <LandingLink href="/login">Начать пользоваться</LandingLink>
        </Reveal>
      </div>
    </section>
  );
}
