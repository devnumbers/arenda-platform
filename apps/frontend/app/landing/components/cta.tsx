"use client";

/* eslint-disable @next/next/no-img-element */

import { useRouter } from "next/navigation";
import { useAuthLanding } from "@/features/auth/lib/use-auth-landing";
import { imgCtaBg } from "./assets";

export function Cta({ openContact }: { openContact: () => void }) {
    const router = useRouter();
    const { ctaHref } = useAuthLanding();
    return (
        <div className="relative shrink-0 w-full px-[20px] tablet:px-[80px] pb-[24px]">
            <div id="cta" className="h-[650px] relative rounded-[32px] desktop:rounded-[64px] shrink-0 w-full overflow-hidden scroll-mt-[100px]">
                <img alt="" className="absolute inset-0 max-w-none object-cover pointer-events-none size-full" src={imgCtaBg} />
                <div className="content-stretch flex flex-col items-center justify-center px-[20px] tablet:px-[80px] relative size-full">
                    <div className="content-stretch flex flex-col gap-[48px] items-center relative shrink-0 w-full max-w-[700px]">
                        <div className="[word-break:break-word] content-stretch flex flex-col gap-[12px] items-center relative shrink-0 text-center text-white w-full">
                            <p className="font-['Involve:SemiBold',sans-serif] leading-[1.15] not-italic relative shrink-0 text-[24px] tablet:text-[36px] desktop:text-[48px] w-full">Попробуйте Рентли в деле</p>
                            <p className="font-['Manrope:Medium',sans-serif] font-medium leading-[1.35] opacity-70 relative shrink-0 text-[16px] desktop:text-[18px] w-full">Добавьте объект и начните отслеживать аренду</p>
                        </div>
                        <div className="content-stretch flex flex-col tablet:flex-row gap-[8px] tablet:gap-[12px] items-stretch tablet:items-center justify-center w-full tablet:w-auto">
                            <button
                                onClick={() => router.push(ctaHref)}
                                className="bg-white content-stretch flex flex-col items-center justify-center min-h-[56px] overflow-clip px-[28px] relative rounded-[16px] shrink-0 w-full tablet:w-auto cursor-pointer transition-colors hover:bg-[#f1f3f6] active:bg-[#e6e9ee]"
                            >
                                <div className="[word-break:break-word] flex flex-col font-['Manrope:Medium',sans-serif] font-medium justify-center leading-[0] overflow-hidden relative shrink-0 text-[#34343c] text-[16px] text-center text-ellipsis whitespace-nowrap">
                                    <p className="leading-[1.35] overflow-hidden text-ellipsis">Начать бесплатно</p>
                                </div>
                            </button>
                            <button
                                onClick={openContact}
                                className="content-stretch flex flex-col items-center justify-center min-h-[56px] overflow-clip px-[28px] relative rounded-[16px] shrink-0 w-full tablet:w-auto cursor-pointer transition-colors hover:bg-white/10 active:bg-white/20"
                            >
                                <div className="[word-break:break-word] flex flex-col font-['Manrope:Medium',sans-serif] font-medium justify-center leading-[0] overflow-hidden relative shrink-0 text-[16px] text-center text-ellipsis text-white whitespace-nowrap">
                                    <p className="leading-[1.35] overflow-hidden text-ellipsis">Связаться с нами</p>
                                </div>
                            </button>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
}
