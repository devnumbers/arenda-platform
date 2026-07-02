"use client";

import { useState } from "react";
import { IconPlus, IconMinus } from "./icons";

const FAQ_ITEMS = [
    {
        q: "Зачем нужен Рентли, если я уже веду аренду в таблице?",
        a: "В таблице нужно самому помнить даты, менять статусы и проверять просрочки. Рентли хранит объекты, арендаторов, договоры и платежи в одном месте, показывает ближайшие оплаты и помогает не пропустить важные даты.",
    },
    {
        q: "Рентли принимает оплату от арендаторов?",
        a: "Нет. Рентли не проводит оплату и не списывает деньги. Вы отмечаете поступления вручную: указываете сумму, дату, объект и арендатора. Так видно, кто уже оплатил, кто задерживает платёж и сколько денег поступило.",
    },
    {
        q: "Нужно ли арендатору регистрироваться в сервисе?",
        a: "Нет. Рентли нужен собственнику или управляющему. Вы можете добавить арендатора в свою базу, хранить его контакты, договор, платежи и историю аренды без регистрации арендатора.",
    },
    {
        q: "Как Рентли помогает не пропустить оплату?",
        a: "Вы указываете дату платежа и настраиваете напоминание. Сервис заранее покажет, что скоро должна поступить оплата. Если дата прошла, а платёж не отмечен как оплаченный, Рентли покажет просрочку.",
    },
    {
        q: "Можно ли понять, сколько объект реально приносит?",
        a: "Да. В Рентли можно учитывать доходы, расходы и прибыль по каждому объекту. Вы увидите, сколько пришло от аренды, сколько ушло на ремонт, коммунальные платежи, налоги или другие расходы, и какая сумма осталась в итоге.",
    },
    {
        q: "Что будет с данными после завершения аренды?",
        a: "История сохранится. Вы сможете завершить аренду, а данные об арендаторе, договоре, платежах и операциях останутся в карточке объекта. Это удобно, если позже нужно проверить прошлые оплаты или условия аренды.",
    },
    {
        q: "Подойдёт ли Рентли, если у меня только один объект?",
        a: "Да. Сервис полезен даже для одной квартиры, комнаты, гаража или помещения. Он помогает помнить даты оплаты, видеть просрочки, хранить данные арендатора и понимать, сколько объект приносит после расходов.",
    },
];

export function Faq() {
    const [openIdx, setOpenIdx] = useState<number | null>(0);
    return (
        <div className="relative shrink-0 w-full">
            <div className="content-stretch flex flex-col items-center px-[20px] tablet:px-[80px] py-[128px] relative size-full">
                <div id="faq" className="content-stretch flex flex-col gap-[32px] desktop:gap-[52px] items-start max-w-[1200px] relative shrink-0 w-full scroll-mt-[100px]">
                    <p className="[word-break:break-word] font-['Involve:SemiBold',sans-serif] leading-[1.15] not-italic relative shrink-0 text-[#34343c] text-[24px] tablet:text-[32px] desktop:text-[48px] text-center w-full">Ответы на частые вопросы</p>
                    <div className="content-stretch flex flex-col gap-[16px] items-start relative shrink-0 w-full">
                        {FAQ_ITEMS.map((item, i) => {
                            const open = openIdx === i;
                            return (
                                <div key={item.q} className="bg-[#f1f3f6] relative rounded-[24px] shrink-0 w-full overflow-clip">
                                    <button
                                        onClick={() => setOpenIdx(open ? null : i)}
                                        className="content-stretch flex flex-col gap-[12px] items-start px-[24px] tablet:px-[32px] py-[24px] relative w-full text-left cursor-pointer transition-colors hover:bg-[#e9ecf1] active:bg-[#e1e5eb]"
                                    >
                                        <div className="content-stretch flex gap-[16px] tablet:gap-[32px] items-center relative shrink-0 w-full">
                                            <p className="[word-break:break-word] flex-[1_0_0] font-['Manrope:Medium',sans-serif] font-medium leading-[1.35] tablet:leading-[32px] min-w-px relative text-[#34343c] text-[16px] tablet:text-[20px] desktop:text-[24px]">{item.q}</p>
                                            {open ? <IconMinus /> : <IconPlus />}
                                        </div>
                                        {open && (
                                            <p className="[word-break:break-word] font-['Manrope:Medium',sans-serif] font-medium leading-[1.35] max-w-[800px] opacity-70 relative shrink-0 text-[#34343c] text-[16px] desktop:text-[18px] w-full">{item.a}</p>
                                        )}
                                    </button>
                                </div>
                            );
                        })}
                    </div>
                </div>
            </div>
        </div>
    );
}
