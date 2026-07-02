/* eslint-disable @next/next/no-img-element */
import { imgFeature1, imgFeature2, imgFeature3, imgFeature4, imgFeature5 } from "../components/assets";
import { IconUser, IconBuilding, IconCase } from "../components/icons";

const FEATURES = [
    {
        img: imgFeature1,
        title: "Добавьте договор аренды, а мы напомним об оплате",
        desc: "Укажите объект, арендатора, срок, сумму и дату оплаты. Рентли сохранит условия аренды и напомнит, когда ждать оплату",
    },
    {
        img: imgFeature2,
        title: "Напомним, когда договор подойдёт к концу",
        desc: "Рентли заранее покажет, что договор подходит к концу. Вы сможете продлить аренду, изменить условия или завершить её, сохранив историю объекта",
    },
    {
        img: imgFeature3,
        title: "Добавляйте платежи и выбирайте, когда о них напоминать",
        desc: "Создайте регулярный платёж один раз: аренду, коммунальные услуги, залог, интернет или другой платёж. Рентли сам добавит следующие оплаты в график",
    },
    {
        img: imgFeature4,
        title: "Уведомим вас, если платёж просрочился",
        desc: "Если дата оплаты прошла, а платёж не отмечен как оплаченный, Рентли покажет просрочку: сумму долга, объект и арендатора",
    },
    {
        img: imgFeature5,
        title: "Отслеживайте доходы и расходы",
        desc: "Добавляйте доходы, расходы, ремонт, налоги и коммунальные платежи. Рентли покажет прибыль по каждому объекту и поможет понять, какая недвижимость зарабатывает больше",
    },
];

function FeatureRow({ img, title, desc, reverse }: { img: string; title: string; desc: string; reverse: boolean }) {
    return (
        <div className={`content-stretch flex flex-col tablet:flex-row items-stretch tablet:items-center justify-center gap-[12px] tablet:gap-[32px] desktop:gap-[64px] relative shrink-0 w-full ${reverse ? "desktop:flex-row-reverse" : ""}`}>
            <div className="aspect-[3360/2520] tablet:flex-[1_0_0] tablet:min-w-px relative rounded-[24px] tablet:rounded-[32px] w-full">
                <img alt="" className="absolute inset-0 max-w-none object-cover pointer-events-none rounded-[24px] tablet:rounded-[32px] size-full" src={img} />
            </div>
            <div className="[word-break:break-word] content-stretch flex flex-col gap-[8px] tablet:gap-[16px] items-start tablet:flex-[1_0_0] tablet:min-w-px overflow-clip relative text-[#34343c] bg-[#f1f3f6] tablet:bg-transparent rounded-[24px] tablet:rounded-none p-[24px] tablet:p-0">
                <p className="font-['Involve:SemiBold',sans-serif] leading-[1.15] not-italic relative shrink-0 text-[20px] tablet:text-[24px] desktop:text-[40px] w-full">{title}</p>
                <p className="font-['Manrope:Medium',sans-serif] font-medium leading-[1.35] opacity-70 relative shrink-0 text-[16px] desktop:text-[18px] w-full">{desc}</p>
            </div>
        </div>
    );
}

function ForWhomCard({
    bg,
    iconWrapBg,
    icon,
    title,
    titleColor,
    desc,
    descColor,
}: {
    bg: string;
    iconWrapBg: string;
    icon: React.ReactNode;
    title: string;
    titleColor: string;
    desc: string;
    descColor: string;
}) {
    return (
        <div className={`${bg} tablet:flex-1 desktop:flex-[1_0_0] desktop:min-w-px desktop:h-[340px] relative rounded-[32px] w-full`}>
            <div className="content-stretch flex flex-col gap-[24px] tablet:gap-[40px] desktop:gap-0 desktop:justify-between items-start p-[24px] tablet:p-[40px] relative size-full desktop:min-h-[340px]">
                <div className={`${iconWrapBg} content-stretch flex items-center justify-center p-[12px] relative rounded-[16px] shrink-0`}>{icon}</div>
                <div className={`[word-break:break-word] content-stretch flex flex-col gap-[8px] tablet:gap-[12px] items-start relative shrink-0 w-full ${titleColor}`}>
                    <p className="font-['Involve:SemiBold',sans-serif] leading-[1.15] not-italic relative shrink-0 text-[24px] tablet:text-[32px] w-full">{title}</p>
                    <p className={`font-['Manrope:Medium',sans-serif] font-medium leading-[1.35] opacity-70 relative shrink-0 text-[16px] desktop:text-[18px] w-full ${descColor}`}>{desc}</p>
                </div>
            </div>
        </div>
    );
}

export function FeaturesAndForWhom() {
    return (
        <div className="relative shrink-0 w-full">
            <div className="content-stretch flex flex-col items-center px-[20px] tablet:px-[80px] py-[128px] relative size-full">
                <div className="content-stretch flex flex-col gap-[128px] items-start max-w-[1200px] relative shrink-0 w-full">
                    {/* Возможности */}
                    <div id="features" className="content-stretch flex flex-col gap-[32px] desktop:gap-[52px] items-start relative shrink-0 w-full scroll-mt-[100px]">
                        <p className="[word-break:break-word] font-['Involve:SemiBold',sans-serif] leading-[1.15] not-italic relative shrink-0 text-[#34343c] text-[24px] tablet:text-[32px] desktop:text-[48px] text-center w-full">Возможности</p>
                        <div className="content-stretch flex flex-col gap-[96px] items-start relative shrink-0 w-full">
                            {FEATURES.map((f, i) => (
                                <FeatureRow key={f.title} img={f.img} title={f.title} desc={f.desc} reverse={i % 2 === 1} />
                            ))}
                        </div>
                    </div>

                    {/* Для кого сервис */}
                    <div id="for-whom" className="content-stretch flex flex-col gap-[32px] desktop:gap-[52px] items-center relative shrink-0 w-full scroll-mt-[100px]">
                        <p className="[word-break:break-word] font-['Involve:SemiBold',sans-serif] leading-[1.15] not-italic relative shrink-0 text-[#34343c] text-[24px] tablet:text-[32px] desktop:text-[48px] text-center w-full">Для кого сервис</p>
                        <div className="content-stretch flex flex-col desktop:flex-row gap-[16px] desktop:gap-[24px] items-start relative shrink-0 w-full">
                            <ForWhomCard
                                bg="bg-[#d9ebff]"
                                iconWrapBg="bg-[#2b7fff]"
                                icon={<IconUser />}
                                title="Собственники"
                                titleColor="text-[#34343c]"
                                desc="Если сдаёте свою квартиру, дом, комнату, гараж или помещение"
                                descColor="text-[#2b7fff]"
                            />
                            <ForWhomCard
                                bg="bg-[#f1f3f6]"
                                iconWrapBg="bg-[#34343c]"
                                icon={<IconBuilding />}
                                title="Агентства"
                                titleColor="text-[#34343c]"
                                desc="Если управляете десятками или сотнями объектов"
                                descColor="text-[#34343c]"
                            />
                            <ForWhomCard
                                bg="bg-[#34343c]"
                                iconWrapBg="bg-white"
                                icon={<IconCase />}
                                title="Бизнес"
                                titleColor="text-white"
                                desc="Если сдаёте офисы, склады, торговые помещения, гаражи или другую коммерческую недвижимость"
                                descColor="text-white"
                            />
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
}
