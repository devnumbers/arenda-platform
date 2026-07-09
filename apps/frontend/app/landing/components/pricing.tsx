"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useAuthLanding } from "@/features/auth/lib/use-auth-landing";
import { ColorIconHome, ColorIconHouses, ColorIconCheck } from "../components/icons";

const ALL_FEATURES = [
    "Напоминания об оплате аренды и платежах",
    "Уведомления об окончании договора",
    "Доходность по каждому объекту",
    "Учёт доходов, расходов и прибыли",
    "Карточки арендаторов с контактами",
    "История платежей по каждому арендатору",
];

function PriceButton({ variant, main, sub, onClick }: { variant: "blue" | "gray"; main: string; sub?: string; onClick?: () => void }) {
    if (variant === "blue") {
        return (
            <button
                onClick={onClick}
                className="bg-[#2b7fff] min-h-[56px] relative rounded-[16px] shrink-0 w-full cursor-pointer transition-colors hover:bg-[#1f6fe8] active:bg-[#1a63d1]"
            >
                <div className="content-stretch flex flex-col items-center justify-center min-h-[56px] px-[24px] relative size-full">
                    <div className="[word-break:break-word] flex flex-col font-['Manrope:Medium',sans-serif] font-medium justify-center leading-[0] overflow-hidden relative shrink-0 text-[16px] text-center text-ellipsis text-white w-full whitespace-nowrap">
                        <p className="leading-[1.35] overflow-hidden text-ellipsis">{main}</p>
                    </div>
                </div>
            </button>
        );
    }
    return (
        <button
            onClick={onClick}
            className="bg-[#f1f3f6] min-h-[56px] relative rounded-[16px] shrink-0 w-full cursor-pointer transition-colors hover:bg-[#e6e9ee] active:bg-[#dce0e6]"
        >
            <div className="[word-break:break-word] content-stretch flex flex-col font-['Manrope:Medium',sans-serif] font-medium items-center justify-center leading-[0] min-h-[56px] px-[24px] py-[8px] relative size-full text-[#34343c] text-center whitespace-nowrap">
                <div className="flex flex-col justify-center overflow-hidden relative shrink-0 text-[16px] text-ellipsis w-full">
                    <p className="leading-[1.35] overflow-hidden text-ellipsis">{main}</p>
                </div>
                {sub && (
                    <div className="flex flex-col justify-center opacity-40 overflow-hidden relative shrink-0 text-[12px] text-ellipsis w-full">
                        <p className="leading-[1.35] overflow-hidden text-ellipsis">{sub}</p>
                    </div>
                )}
            </div>
        </button>
    );
}

function PriceCard({
    plan,
    priceTop,
    objects,
    objectsIcon,
    button,
}: {
    plan: string;
    priceTop: React.ReactNode;
    objects: string;
    objectsIcon: React.ReactNode;
    button: React.ReactNode;
}) {
    return (
        <div className="bg-white desktop:flex-[1_0_0] desktop:min-w-px desktop:min-h-[428px] relative rounded-[32px] w-full self-stretch">
            <div className="content-stretch flex flex-col gap-[24px] desktop:gap-0 desktop:justify-between items-start p-[24px] tablet:p-[40px] relative size-full desktop:min-h-[428px]">
                <div className="content-stretch flex flex-col gap-[40px] items-start relative shrink-0">
                    <p className="[word-break:break-word] font-['Manrope:Medium',sans-serif] font-medium leading-[1.35] opacity-70 relative shrink-0 text-[#34343c] text-[24px] whitespace-nowrap">{plan}</p>
                    {priceTop}
                </div>
                <div className="content-stretch flex gap-[12px] items-center relative shrink-0">
                    {objectsIcon}
                    <p className="[word-break:break-word] font-['Manrope:SemiBold',sans-serif] font-semibold leading-[1.35] relative shrink-0 text-[#2b7fff] text-[24px] whitespace-nowrap">{objects}</p>
                </div>
                {button}
            </div>
        </div>
    );
}

// eslint-disable-next-line @typescript-eslint/no-unused-vars
export function Pricing({ openContact }: { openContact: () => void }) {
    const router = useRouter();
    const { ctaHref } = useAuthLanding();
    const [yearly, setYearly] = useState(false);

    const proPrice = yearly ? "4 400 ₽" : "490 ₽";
    const proPeriod = yearly ? "в год" : "в месяц";
    const bizPrice = yearly ? "8 900 ₽" : "990 ₽";
    const bizPeriod = yearly ? "в год" : "в месяц";

    const proButton = yearly
        ? { main: "Подключить за 367 ₽ в месяц", sub: "При оплате за год" }
        : { main: "Подключить за 490 ₽ в месяц", sub: undefined };
    const bizButton = yearly
        ? { main: "Подключить за 742 ₽ в месяц", sub: "При оплате за год" }
        : { main: "Подключить за 990 ₽ в месяц", sub: undefined };

    const toggleBtnBase =
        "content-stretch flex items-center justify-center min-h-[56px] px-[28px] relative rounded-[16px] shrink-0 cursor-pointer transition-colors";

    return (
        <div className="relative shrink-0 w-full px-[20px] tablet:px-[40px] desktop:px-[80px] py-[24px]">
            <div id="pricing" className="bg-[#f1f3f6] relative rounded-[64px] shrink-0 w-full scroll-mt-[100px]">
                <div className="content-stretch flex flex-col items-center px-[20px] tablet:px-[40px] desktop:px-[80px] py-[128px] relative size-full">
                    <div className="content-stretch flex flex-col gap-[32px] desktop:gap-[52px] items-center max-w-[1200px] relative shrink-0 w-full">
                        <p className="[word-break:break-word] font-['Involve:SemiBold',sans-serif] leading-[1.15] not-italic relative shrink-0 text-[#34343c] text-[24px] tablet:text-[32px] desktop:text-[48px] text-center w-full">Выберите подходящий тариф</p>

                        <div className="content-stretch flex flex-col gap-[24px] items-center relative shrink-0 w-full">
                            {/* Toggle */}
                            <div className="bg-white content-stretch flex gap-[4px] items-stretch overflow-clip p-[4px] relative rounded-[20px] shrink-0">
                                <button
                                    onClick={() => setYearly(false)}
                                    className={`${toggleBtnBase} ${!yearly ? "bg-[#f1f3f6]" : "hover:bg-[#f6f7f9]"} active:bg-[#e6e9ee]`}
                                >
                                    <div className="[word-break:break-word] flex flex-col font-['Manrope:Medium',sans-serif] font-medium justify-center leading-[0] overflow-hidden relative shrink-0 text-[#34343c] text-[16px] text-center text-ellipsis whitespace-nowrap">
                                        <p className="leading-[1.35] overflow-hidden text-ellipsis">В месяц</p>
                                    </div>
                                </button>
                                <button
                                    onClick={() => setYearly(true)}
                                    className={`${toggleBtnBase} gap-[12px] ${yearly ? "bg-[#f1f3f6]" : "hover:bg-[#f6f7f9]"} active:bg-[#e6e9ee]`}
                                >
                                    <div className="[word-break:break-word] flex flex-col font-['Manrope:Medium',sans-serif] font-medium justify-center leading-[0] overflow-hidden relative shrink-0 text-[#34343c] text-[16px] text-center text-ellipsis whitespace-nowrap">
                                        <p className="leading-[1.35] overflow-hidden text-ellipsis">В год</p>
                                    </div>
                                    <div className="bg-[#2b7fff] content-stretch flex items-center justify-center px-[8px] py-[4px] relative rounded-[45px] shrink-0">
                                        <div className="[word-break:break-word] flex flex-col font-['Manrope:Medium',sans-serif] font-medium justify-center leading-[0] overflow-hidden relative shrink-0 text-[14px] text-center text-ellipsis text-white whitespace-nowrap">
                                            <p className="leading-[1.35] overflow-hidden text-ellipsis">— 25%</p>
                                        </div>
                                    </div>
                                </button>
                            </div>

                            {/* Cards */}
                            <div className="content-stretch flex flex-col desktop:flex-row gap-[16px] desktop:gap-[24px] desktop:h-[428px] items-stretch relative shrink-0 w-full">
                                <PriceCard
                                    plan="Базовый"
                                    priceTop={
                                        <div className="content-stretch flex h-[78px] items-start min-h-[78px] relative shrink-0">
                                            <p className="[word-break:break-word] font-['Manrope:Bold',sans-serif] font-bold leading-[1.15] relative shrink-0 text-[#34343c] text-[40px] whitespace-nowrap">Бесплатно</p>
                                        </div>
                                    }
                                    objects="1 объект"
                                    objectsIcon={<ColorIconHome />}
                                    button={<PriceButton variant="blue" main="Попробовать" onClick={() => router.push(ctaHref)} />}
                                />
                                <PriceCard
                                    plan="Про"
                                    priceTop={
                                        <div className="content-stretch flex flex-col gap-[8px] h-[78px] items-start min-h-[78px] relative shrink-0 text-[#34343c]">
                                            <div className="content-stretch flex gap-[8px] items-end relative shrink-0">
                                                <p className="font-['Manrope:Bold',sans-serif] font-bold leading-[1.15] relative shrink-0 text-[40px]">{proPrice}</p>
                                                <p className="font-['Manrope:Medium',sans-serif] font-medium leading-[1.35] opacity-40 relative shrink-0 text-[18px]">{proPeriod}</p>
                                            </div>
                                            {yearly && <p className="font-['Manrope:Medium',sans-serif] font-medium leading-[1.35] opacity-40 relative shrink-0 text-[18px]">Экономия 1 480 ₽</p>}
                                        </div>
                                    }
                                    objects="5 объектов"
                                    objectsIcon={<ColorIconHouses />}
                                    button={<PriceButton variant="gray" main={proButton.main} sub={proButton.sub} onClick={() => router.push(ctaHref)} />}
                                />
                                <PriceCard
                                    plan="Бизнес"
                                    priceTop={
                                        <div className="content-stretch flex flex-col gap-[8px] h-[78px] items-start min-h-[78px] relative shrink-0 text-[#34343c]">
                                            <div className="content-stretch flex gap-[8px] items-end relative shrink-0">
                                                <p className="font-['Manrope:Bold',sans-serif] font-bold leading-[1.15] relative shrink-0 text-[40px]">{bizPrice}</p>
                                                <p className="font-['Manrope:Medium',sans-serif] font-medium leading-[1.35] opacity-40 relative shrink-0 text-[18px]">{bizPeriod}</p>
                                            </div>
                                            {yearly && <p className="font-['Manrope:Medium',sans-serif] font-medium leading-[1.35] opacity-40 relative shrink-0 text-[18px]">Экономия 2 980 ₽</p>}
                                        </div>
                                    }
                                    objects="Без ограничений"
                                    objectsIcon={<ColorIconHouses />}
                                    button={<PriceButton variant="gray" main={bizButton.main} sub={bizButton.sub} onClick={() => router.push(ctaHref)} />}
                                />
                            </div>

                            {/* All features */}
                            <div className="bg-white relative rounded-[32px] shrink-0 w-full overflow-clip">
                                <div className="content-stretch flex flex-col gap-[32px] items-start p-[24px] tablet:p-[40px] relative size-full">
                                    <p className="[word-break:break-word] font-['Manrope:Medium',sans-serif] font-medium leading-[1.35] relative shrink-0 text-[#2b7fff] text-[24px] w-full">Каждый тариф включает все функции сервиса</p>
                                    <div className="content-stretch flex flex-col gap-[24px] items-start relative shrink-0 w-full">
                                        {ALL_FEATURES.map((f) => (
                                            <div key={f} className="content-stretch flex gap-[12px] items-center relative shrink-0 w-full">
                                                <ColorIconCheck />
                                                <p className="[word-break:break-word] flex-[1_0_0] font-['Manrope:Medium',sans-serif] font-medium leading-[32px] min-w-px relative text-[#34343c] text-[18px] tablet:text-[24px]">{f}</p>
                                            </div>
                                        ))}
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
}
