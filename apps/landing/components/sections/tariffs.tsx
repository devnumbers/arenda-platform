import { Reveal } from "@/components/reveal";
import { TariffCards } from "@/components/sections/tariff-cards";
import { TARIFFS } from "@/lib/content";
import { getMe } from "@/lib/auth";

// «Тарифы» — макет 2846-153710. Для авторизованного кнопка его тарифа
// превращается в «Ваш тариф» (требование владельца, карта #888).
export async function Tariffs() {
  const me = await getMe();

  return (
    <section id="tariffs" className="mt-20 desk:mt-[156px]">
      <div className="mx-auto flex w-full max-w-[1000px] flex-col items-center px-5 desk:px-10">
        <Reveal className="w-full">
          <h2 className="text-center text-[28px] font-semibold leading-8 desk:text-h2 desk:leading-[60px]">
            Тарифы
          </h2>
        </Reveal>
        <Reveal delay={100} className="w-full">
          <div className="mt-14 flex w-full flex-col items-center gap-14">
            <TariffCards tariffs={TARIFFS} currentTariff={me?.tariff} />
          </div>
        </Reveal>
      </div>
    </section>
  );
}
