"use client";

/* eslint-disable @next/next/no-img-element -- preserving original landing <img> tags during migration */

import { useState } from "react";
import { useRouter, usePathname } from "next/navigation";
import { imgLogo } from "./assets";
import { IconMenu, IconClose } from "./icons";
import { ROUTES } from "@/shared/config/routes";

const NAV_ITEMS: { label: string; target: string }[] = [
    { label: "Возможности", target: "features" },
    { label: "Для кого", target: "for-whom" },
    { label: "Цены", target: "pricing" },
    { label: "Вопросы", target: "faq" },
    { label: "Поддержка", target: "cta" },
];

function Logo() {
    return (
        <div className="content-stretch flex gap-[10px] items-center relative shrink-0">
            <div className="relative shrink-0 size-[32px]" data-name="logo">
                <img alt="Рентли" className="absolute inset-0 max-w-none object-cover pointer-events-none size-full" src={imgLogo} />
            </div>
            <div className="[word-break:break-word] flex flex-col font-['Involve:Bold',sans-serif] justify-center leading-[0] not-italic relative shrink-0 text-[#2b7fff] text-[24px] text-center whitespace-nowrap">
                <p className="leading-[0.6]">рентли</p>
            </div>
        </div>
    );
}

export function Header() {
    const [open, setOpen] = useState(false);
    const router = useRouter();
    const pathname = usePathname();

    const goToSection = (target: string) => {
        setOpen(false);
        if (pathname && pathname !== "/") {
            router.push("/");
            window.setTimeout(() => {
                document.getElementById(target)?.scrollIntoView({ behavior: "smooth" });
            }, 60);
        } else {
            document.getElementById(target)?.scrollIntoView({ behavior: "smooth" });
        }
    };

    const goLogin = () => {
        setOpen(false);
        router.push(ROUTES.dashboard);
    };

    return (
        <div className="absolute content-stretch flex flex-col items-center justify-center left-0 pt-[16px] px-[16px] right-0 top-0 z-50">
            <div className="bg-white max-w-[1200px] relative rounded-[24px] shrink-0 w-full">
                <div className="flex flex-row items-center max-w-[inherit] size-full">
                    <div className="content-stretch flex items-center justify-between max-w-[inherit] pl-[20px] pr-[12px] py-[12px] relative size-full">
                        {/* Logo */}
                        <div className="flex flex-[1_0_0] flex-row items-center self-stretch">
                            <div className="content-stretch flex flex-[1_0_0] flex-col h-full items-start justify-center min-w-px relative">
                                <button onClick={() => goToSection("top")} className="cursor-pointer transition-opacity hover:opacity-80 active:opacity-60">
                                    <Logo />
                                </button>
                            </div>
                        </div>

                        {/* Desktop nav */}
                        <div className="hidden desktop:flex content-stretch gap-[32px] items-center relative shrink-0">
                            {NAV_ITEMS.map((item) => (
                                <button
                                    key={item.target}
                                    onClick={() => goToSection(item.target)}
                                    className="content-stretch flex items-center justify-center py-[10px] relative shrink-0 cursor-pointer transition-colors hover:text-[#2b7fff] active:text-[#1a63d1]"
                                >
                                    <div className="[word-break:break-word] flex flex-col font-['Manrope:Medium',sans-serif] font-medium justify-center leading-[0] relative shrink-0 text-[16px] text-center whitespace-nowrap">
                                        <p className="leading-[1.35]">{item.label}</p>
                                    </div>
                                </button>
                            ))}
                        </div>

                        {/* Desktop login */}
                        <div className="hidden desktop:flex content-stretch flex-[1_0_0] flex-col items-end justify-center min-w-px relative">
                            <button
                                onClick={goLogin}
                                className="bg-[#f1f3f6] content-stretch flex flex-col items-start justify-center min-h-[44px] overflow-clip px-[20px] relative rounded-[12px] shrink-0 cursor-pointer transition-colors hover:bg-[#e6e9ee] active:bg-[#dce0e6]"
                            >
                                <div className="[word-break:break-word] flex flex-col font-['Manrope:Medium',sans-serif] font-medium justify-center leading-[0] overflow-hidden relative shrink-0 text-[#34343c] text-[16px] text-center text-ellipsis whitespace-nowrap">
                                    <p className="leading-[1.35] overflow-hidden text-ellipsis">Войти</p>
                                </div>
                            </button>
                        </div>

                        {/* Tablet: login + burger */}
                        <div className="hidden tablet:flex desktop:hidden content-stretch flex-[1_0_0] gap-[4px] items-center justify-end min-w-px relative">
                            <button
                                onClick={goLogin}
                                className="bg-[#f1f3f6] content-stretch flex flex-col items-start justify-center min-h-[44px] overflow-clip px-[20px] relative rounded-[12px] shrink-0 cursor-pointer transition-colors hover:bg-[#e6e9ee] active:bg-[#dce0e6]"
                            >
                                <div className="[word-break:break-word] flex flex-col font-['Manrope:Medium',sans-serif] font-medium justify-center leading-[0] overflow-hidden relative shrink-0 text-[#34343c] text-[16px] text-center text-ellipsis whitespace-nowrap">
                                    <p className="leading-[1.35] overflow-hidden text-ellipsis">Войти</p>
                                </div>
                            </button>
                            <BurgerButton open={open} onClick={() => setOpen((v) => !v)} />
                        </div>

                        {/* Mobile: burger only */}
                        <div className="flex tablet:hidden content-stretch flex-[1_0_0] items-center justify-end min-w-px relative">
                            <BurgerButton open={open} onClick={() => setOpen((v) => !v)} />
                        </div>
                    </div>
                </div>

                {/* Burger dropdown menu */}
                {open && (
                    <div className="desktop:hidden absolute left-0 right-0 top-[calc(100%+8px)] bg-white rounded-[24px] p-[12px] shadow-[0px_8px_32px_0px_rgba(43,127,255,0.12)]">
                        <div className="content-stretch flex flex-col items-stretch">
                            {NAV_ITEMS.map((item) => (
                                <button
                                    key={item.target}
                                    onClick={() => goToSection(item.target)}
                                    className="content-stretch flex items-center px-[8px] py-[14px] relative shrink-0 cursor-pointer rounded-[12px] transition-colors hover:bg-[#f1f3f6] active:bg-[#e6e9ee]"
                                >
                                    <div className="[word-break:break-word] flex flex-col font-['Manrope:Medium',sans-serif] font-medium justify-center leading-[0] relative shrink-0 text-[#34343c] text-[16px] whitespace-nowrap">
                                        <p className="leading-[1.35]">{item.label}</p>
                                    </div>
                                </button>
                            ))}
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
}

function BurgerButton({ open, onClick }: { open: boolean; onClick: () => void }) {
    return (
        <button
            onClick={onClick}
            aria-label="Меню"
            className="bg-[#f1f3f6] content-stretch flex flex-col items-center justify-center min-h-[44px] min-w-[44px] overflow-clip relative rounded-[12px] shrink-0 cursor-pointer transition-colors hover:bg-[#e6e9ee] active:bg-[#dce0e6]"
        >
            {open ? <IconClose /> : <IconMenu />}
        </button>
    );
}
