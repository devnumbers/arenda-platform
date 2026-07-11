import { useState } from "react";
import { PlusIcon } from "./icons";
import { typo } from "./typo";

const FAQ_ITEMS: { q: string; a: string }[] = [
  {
    q: "Зачем мне Рентли, если я уже веду аренду в таблице?",
    a: "В таблице нужно самому помнить даты, менять статусы и проверять просрочки. Рентли хранит объекты, арендаторов, договоры и платежи в одном месте, показывает ближайшие оплаты и помогает не пропустить важные даты",
  },
  {
    q: "Сервис сам принимает оплату от арендаторов?",
    a: "Нет, Рентли не проводит платежи. Сервис помогает вести учёт: фиксирует суммы, сроки и статусы оплат, а также напоминает о ближайших и просроченных платежах",
  },
  {
    q: "Нужно ли арендатору регистрироваться?",
    a: "Нет. Все данные ведёте вы. Арендатору не нужно создавать аккаунт или устанавливать приложение — достаточно ваших записей в сервисе",
  },
  {
    q: "Как Рентли помогает не пропустить оплату?",
    a: "Сервис показывает ближайшие оплаты и автоматически напоминает о датах. Если платёж просрочен, вы увидите сумму и объект в списке",
  },
  {
    q: "Будет ли видно, сколько объект реально приносит денег?",
    a: "Да. Добавляйте доходы и расходы по каждому объекту, и Рентли покажет прибыль, чтобы вы понимали реальную доходность аренды",
  },
  {
    q: "Что происходит с данными после завершения аренды?",
    a: "Данные сохраняются в вашей истории. Вы можете вернуться к завершённым договорам, посмотреть платежи и статистику или удалить записи при необходимости",
  },
  {
    q: "Подойдет ли Рентли, если у меня один объект?",
    a: "Да. Сервис одинаково удобен и для одного объекта, и для десятков. Вы ведёте договор, платежи и напоминания в одном месте независимо от количества объектов",
  },
];

export function Faq() {
  const [openIndex, setOpenIndex] = useState<number>(0);

  return (
    <div className="flex flex-col gap-[8px] min-[768px]:gap-[16px] items-start w-full">
      {FAQ_ITEMS.map((item, i) => {
        const open = openIndex === i;
        return (
          <div key={item.q} className="bg-[#f1f3f6] rounded-[24px] w-full overflow-hidden">
            <button
              onClick={() => setOpenIndex(open ? -1 : i)}
              className="w-full flex gap-[32px] items-center text-left px-[32px] py-[24px] cursor-pointer max-[767px]:px-[24px] max-[767px]:py-[20px] max-[767px]:gap-[16px]"
            >
              <p className="flex-1 min-w-px font-['Manrope:Medium',sans-serif] font-medium leading-[1.35] text-[#222] text-[24px] max-[767px]:text-[18px]">
                {typo(item.q)}
              </p>
              <PlusIcon open={open} />
            </button>
            <div
              className="grid transition-[grid-template-rows] duration-200 ease-out"
              style={{ gridTemplateRows: open ? "1fr" : "0fr" }}
            >
              <div className="overflow-hidden">
                <p className="font-['Manrope:Medium',sans-serif] font-medium leading-[1.35] max-w-[800px] opacity-70 text-[#222] text-[18px] px-[32px] pb-[24px] max-[767px]:px-[24px] max-[767px]:pb-[20px] max-[767px]:text-[14px]">
                  {typo(item.a)}
                </p>
              </div>
            </div>
          </div>
        );
      })}
    </div>
  );
}
