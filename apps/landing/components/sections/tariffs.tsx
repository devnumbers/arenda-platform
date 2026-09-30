import type { ReactNode } from "react";
import { Reveal } from "@/components/reveal";
import { TariffCards } from "@/components/sections/tariff-cards";
import { TARIFFS } from "@/lib/content";
import { getMe } from "@/lib/auth";

// Обвязка секции: контент и fallback рендерятся в ней, поэтому геометрия
// двух состояний совпадает структурно, а не по копипасте (как HeaderShell).
// Якорь #tariffs есть и в fallback — пункт «Тарифы» навигации работает
// и до подмены контента.
function TariffsShell({ children }: { children: ReactNode }) {
  return (
    <section id="tariffs" className="mt-24 scroll-mt-[88px] desk:mt-[156px] desk:scroll-mt-[104px]">
      <div className="mx-auto flex w-full max-w-[1048px] flex-col items-center px-6 desk:max-w-[1000px] desk:px-0">
        {children}
      </div>
    </section>
  );
}

// «Тарифы» — макет 2846-153710. Для авторизованного кнопка его тарифа
// превращается в «Ваш тариф» (требование владельца, карта #888).
export async function Tariffs() {
  const me = await getMe();

  return (
    <TariffsShell>
      <Reveal delay={100} className="flex w-full flex-col items-center">
        <TariffCards tariffs={TARIFFS} currentTariff={me?.tariff} />
      </Reveal>
    </TariffsShell>
  );
}

// Fallback Suspense-дырки страницы (ADR 0063 — «дырка хедера под <Suspense>»):
// гостевой вид секции, той же геометрии — авторизованному кнопка «Ваш тариф»
// подменяется без сдвига макета, shell страницы фетчем /me не задерживается.
export function TariffsFallback() {
  return (
    <TariffsShell>
      <Reveal delay={100} className="flex w-full flex-col items-center">
        <TariffCards tariffs={TARIFFS} />
      </Reveal>
    </TariffsShell>
  );
}
