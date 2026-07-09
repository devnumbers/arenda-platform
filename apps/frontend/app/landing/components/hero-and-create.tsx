"use client";

/* eslint-disable @next/next/no-img-element -- preserving original landing <img> tags during migration */

import { useRouter } from "next/navigation";
import { ROUTES } from "@/shared/config/routes";
import { imgHeroBg, imgLogo3D, imgSetting01, imgPhoneFrame } from "./assets";

function PrimaryButton({ children, className = "", onClick }: { children: React.ReactNode; className?: string; onClick?: () => void }) {
    return (
        <button
            onClick={onClick}
            className={`bg-[#2b7fff] content-stretch flex flex-col items-center justify-center overflow-clip px-[28px] py-[18px] relative rounded-[16px] shrink-0 cursor-pointer transition-colors hover:bg-[#1f6fe8] active:bg-[#1a63d1] ${className}`}
        >
            <div className="[word-break:break-word] flex flex-col font-['Manrope:Medium',sans-serif] font-medium justify-center leading-[0] overflow-hidden relative shrink-0 text-[16px] text-center text-ellipsis text-white whitespace-nowrap">
                <p className="leading-[20px] overflow-hidden text-ellipsis">{children}</p>
            </div>
        </button>
    );
}

function GhostBlueButton({ children, className = "", onClick }: { children: React.ReactNode; className?: string; onClick?: () => void }) {
    return (
        <button
            onClick={onClick}
            className={`content-stretch flex flex-col items-center justify-center overflow-clip px-[28px] py-[18px] relative rounded-[16px] shrink-0 cursor-pointer transition-colors hover:bg-[#eaf2ff] active:bg-[#d9ebff] ${className}`}
        >
            <div className="[word-break:break-word] flex flex-col font-['Manrope:Medium',sans-serif] font-medium justify-center leading-[0] overflow-hidden relative shrink-0 text-[#2b7fff] text-[16px] text-center text-ellipsis whitespace-nowrap">
                <p className="leading-[20px] overflow-hidden text-ellipsis">{children}</p>
            </div>
        </button>
    );
}

export function HeroAndCreate() {
    const router = useRouter();
    return (
        <div id="top" className="content-stretch flex flex-col items-start relative rounded-bl-[32px] rounded-br-[32px] desktop:rounded-bl-[64px] desktop:rounded-br-[64px] shrink-0 w-full overflow-hidden">
            <img alt="" className="absolute inset-0 max-w-none object-cover pointer-events-none rounded-bl-[32px] rounded-br-[32px] desktop:rounded-bl-[64px] desktop:rounded-br-[64px] size-full" src={imgHeroBg} />

            {/* Hero */}
            <div className="relative shrink-0 w-full">
                <div className="content-stretch flex flex-col items-center justify-center px-[40px] tablet:px-[80px] pt-[112px] pb-[64px] tablet:py-[120px] desktop:py-[64px] desktop:min-h-[824px] relative size-full">
                    <div className="content-stretch flex flex-col desktop:flex-row items-center desktop:justify-between gap-[32px] desktop:gap-[40px] max-w-[1200px] relative shrink-0 w-full">
                        <div className="relative shrink-0 size-[96px] tablet:size-[256px] desktop:size-[350px] order-first desktop:order-last" data-name="logo-3d">
                            <img alt="" className="absolute inset-0 max-w-none object-cover pointer-events-none size-full" src={imgLogo3D} />
                        </div>
                        <div className="content-stretch flex flex-col gap-[40px] items-center desktop:items-start max-w-[700px] desktop:flex-[1_0_0] min-w-px relative w-full">
                            <div className="[word-break:break-word] content-stretch flex flex-col gap-[12px] desktop:gap-[24px] items-start relative shrink-0 text-[#34343c] w-full text-center desktop:text-left">
                                <p className="font-['Involve:Bold',sans-serif] leading-[1.15] not-italic relative shrink-0 text-[24px] tablet:text-[40px] desktop:text-[52px] w-full">Рентли — сервис управления арендой недвижимости</p>
                                <p className="font-['Manrope:Medium',sans-serif] font-medium leading-[1.35] opacity-70 desktop:opacity-100 relative shrink-0 text-[14px] tablet:text-[24px] desktop:text-[24px] w-full">Управляйте объектами, арендаторами, платежами и договорами в одном месте</p>
                            </div>
                            <div className="content-stretch flex flex-col tablet:flex-row gap-[12px] items-stretch tablet:items-start w-full tablet:w-auto">
                                <PrimaryButton className="w-full tablet:w-auto" onClick={() => router.push(ROUTES.dashboard)}>Попробовать бесплатно</PrimaryButton>
                                <GhostBlueButton className="w-full tablet:w-auto" onClick={() => router.push(ROUTES.dashboard)}>Войти в сервис</GhostBlueButton>
                            </div>
                        </div>
                    </div>
                </div>
            </div>

            {/* Create card */}
            <div className="relative shrink-0 w-full">
                <div className="content-stretch flex flex-col items-center justify-center pb-[128px] px-[20px] tablet:px-[40px] desktop:px-[128px] relative size-full">
                    <div className="content-stretch flex flex-col gap-[32px] desktop:gap-[52px] items-center desktop:items-start max-w-[1200px] relative shrink-0 w-full">
                        <p className="[word-break:break-word] font-['Involve:SemiBold',sans-serif] leading-[1.15] not-italic relative shrink-0 text-[#34343c] text-[24px] tablet:text-[32px] desktop:text-[48px] text-center w-full">Создайте карточку своей недвижимости и управляйте арендой</p>
                        {/* tablet + desktop: dashboard screenshot with floating phone on desktop */}
                        <div className="hidden tablet:block aspect-[1200/750] relative rounded-[32px] shadow-[0px_8px_64px_0px_rgba(43,127,255,0.24)] shrink-0 w-full" data-name="setting-01">
                            <img alt="" className="absolute inset-0 max-w-none object-cover pointer-events-none rounded-[32px] size-full" src={imgSetting01} />
                            <div className="hidden desktop:block absolute bottom-0 h-[560px] right-[-40px] shadow-[0px_8px_32px_0px_rgba(43,127,255,0.24)] w-[252px]" data-name="phone">
                                <img alt="" className="absolute inset-0 max-w-none object-cover pointer-events-none size-full" src={imgPhoneFrame} />
                            </div>
                        </div>
                        {/* mobile: vertical phone frame */}
                        <div className="block tablet:hidden w-full px-[12px]">
                            <div className="aspect-[1843/4096] relative shadow-[0px_8px_32px_0px_rgba(43,127,255,0.24)] w-full" data-name="phone-mobile">
                                <img alt="" className="absolute inset-0 max-w-none object-cover pointer-events-none size-full" src={imgPhoneFrame} />
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
}
