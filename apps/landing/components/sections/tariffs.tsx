import { Reveal } from "@/components/reveal";
import { TariffCards } from "@/components/sections/tariff-cards";
import { TARIFFS } from "@/lib/content";
import { getMe } from "@/lib/auth";

// «Тарифы» — макет 2846-153710. Для авторизованного кнопка его тарифа
// превращается в «Ваш тариф» (требование владельца, карта #888).
export async function Tariffs() {
  const me = await getMe();

  return (
    <section id="tariffs" className="mt-24 scroll-mt-[88px] desk:mt-[156px] desk:scroll-mt-[104px]">
      <div className="mx-auto flex w-full max-w-[1048px] flex-col items-center px-6 desk:max-w-[1000px] desk:px-0">
        <Reveal delay={100} className="flex w-full flex-col items-center">
          <TariffCards tariffs={TARIFFS} currentTariff={me?.tariff} />
        </Reveal>
      </div>
    </section>
  );
}
