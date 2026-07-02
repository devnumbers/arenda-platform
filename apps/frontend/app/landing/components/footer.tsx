"use client";

/* eslint-disable @next/next/no-img-element */

import { useRouter } from "next/navigation";
import { imgLogo } from "./assets";
import { IconTelegram, IconLetter, IconComment } from "./icons";

function FooterContactButton({ icon, label, onClick }: { icon: React.ReactNode; label: string; onClick?: () => void }) {
    return (
        <button
            onClick={onClick}
            className="bg-[#f1f3f6] content-stretch flex gap-[8px] items-center justify-center min-h-[56px] overflow-clip pl-[24px] pr-[28px] relative rounded-[16px] shrink-0 w-full tablet:w-auto cursor-pointer transition-colors hover:bg-[#e6e9ee] active:bg-[#dce0e6]"
        >
            {icon}
            <div className="[word-break:break-word] flex flex-col font-['Manrope:Medium',sans-serif] font-medium justify-center leading-[0] overflow-hidden relative shrink-0 text-[#34343c] text-[16px] text-center text-ellipsis whitespace-nowrap">
                <p className="leading-[1.35] overflow-hidden text-ellipsis">{label}</p>
            </div>
        </button>
    );
}

export function Footer({ openContact }: { openContact: () => void }) {
    const router = useRouter();

    const goTop = (path: string) => {
        router.push(path);
        window.scrollTo({ top: 0 });
    };

    return (
        <div className="relative shrink-0 w-full bg-white">
            <div className="content-stretch flex flex-col items-center pt-[80px] pb-[20px] px-[20px] tablet:p-[80px] relative size-full">
                <div className="content-stretch flex flex-col gap-[32px] tablet:gap-[64px] items-start max-w-[1200px] relative shrink-0 w-full">
                    {/* Logo */}
                    <div className="content-stretch flex gap-[20px] items-center relative shrink-0 w-full">
                        <div className="relative shrink-0 size-[32px] desktop:size-[64px]" data-name="logo">
                            <img alt="Рентли" className="absolute inset-0 max-w-none object-cover pointer-events-none size-full" src={imgLogo} />
                        </div>
                        <div className="[word-break:break-word] flex flex-col font-['Involve:Bold',sans-serif] justify-center leading-[0] not-italic relative shrink-0 text-[#2b7fff] text-[24px] desktop:text-[48px] text-center whitespace-nowrap">
                            <p className="leading-[0.6]">рентли</p>
                        </div>
                    </div>

                    <div className="content-stretch flex flex-col gap-[52px] tablet:gap-[72px] items-start relative shrink-0 w-full">
                        {/* Contact buttons */}
                        <div className="content-stretch flex flex-col tablet:flex-row gap-[8px] tablet:gap-[12px] items-stretch tablet:items-center relative shrink-0 w-full tablet:w-auto">
                            <FooterContactButton icon={<IconTelegram />} label="Написать в Telegram" onClick={openContact} />
                            <FooterContactButton icon={<IconLetter />} label="hello@rentlee.ru" onClick={openContact} />
                            <FooterContactButton icon={<IconComment />} label="Связаться с нами" onClick={openContact} />
                        </div>

                        {/* Bottom row */}
                        <div className="[word-break:break-word] content-stretch flex flex-col tablet:flex-row font-['Manrope:Medium',sans-serif] font-medium gap-[12px] tablet:gap-[32px] items-start leading-[0] relative shrink-0 text-[#34343c] text-[16px] text-center w-full whitespace-nowrap">
                            <button
                                onClick={() => goTop("/privacy")}
                                className="flex flex-col justify-center relative shrink-0 tablet:order-2 cursor-pointer transition-colors hover:text-[#2b7fff] active:text-[#1a63d1]"
                            >
                                <p className="leading-[1.35]">Политика конфиденциальности</p>
                            </button>
                            <button
                                onClick={() => goTop("/terms")}
                                className="flex flex-col justify-center relative shrink-0 tablet:order-3 cursor-pointer transition-colors hover:text-[#2b7fff] active:text-[#1a63d1]"
                            >
                                <p className="leading-[1.35]">Пользовательское соглашение</p>
                            </button>
                            <div className="flex flex-col justify-center opacity-40 relative shrink-0 tablet:order-1">
                                <p className="leading-[1.35]">© 2026 Рентли</p>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
}
