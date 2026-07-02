"use client";

import { useState } from "react";
import { IconClose } from "./icons";

function ContactModalForm({ onClose }: { onClose: () => void }) {
    const [sent, setSent] = useState(false);
    const [name, setName] = useState("");
    const [email, setEmail] = useState("");
    const [phone, setPhone] = useState("");

    const submit = (e: React.FormEvent) => {
        e.preventDefault();
        setSent(true);
    };

    const inputClass =
        "bg-[#f1f3f6] rounded-[16px] px-[20px] min-h-[56px] w-full font-['Manrope:Medium',sans-serif] font-medium text-[16px] text-[#34343c] placeholder:text-[#34343c] placeholder:opacity-40 outline-none focus:ring-2 focus:ring-[#2b7fff]/30 transition-shadow";

    return (
        <div
            className="fixed inset-0 z-[100] flex items-center justify-center p-[20px] bg-black/40"
            onClick={onClose}
        >
            <div
                className="bg-white rounded-[32px] w-full max-w-[480px] p-[32px] tablet:p-[40px] relative content-stretch flex flex-col gap-[24px] max-h-[90vh] overflow-auto"
                onClick={(e) => e.stopPropagation()}
            >
                <button
                    onClick={onClose}
                    aria-label="Закрыть"
                    className="absolute top-[24px] right-[24px] bg-[#f1f3f6] flex items-center justify-center size-[36px] rounded-[12px] cursor-pointer transition-colors hover:bg-[#e6e9ee] active:bg-[#dce0e6]"
                >
                    <IconClose />
                </button>

                {!sent ? (
                    <>
                        <div className="content-stretch flex flex-col gap-[8px] items-start pr-[40px]">
                            <p className="font-['Involve:SemiBold',sans-serif] leading-[1.15] not-italic text-[#34343c] text-[28px]">Свяжемся с вами</p>
                            <p className="font-['Manrope:Medium',sans-serif] font-medium leading-[1.35] opacity-70 text-[#34343c] text-[16px]">Оставьте имя и контактные данные. Мы ответим на почту или позвоним, чтобы помочь с вопросом по сервису.</p>
                        </div>
                        <form className="content-stretch flex flex-col gap-[12px] w-full" onSubmit={submit}>
                            <input className={inputClass} placeholder="Имя" value={name} onChange={(e) => setName(e.target.value)} required />
                            <input className={inputClass} type="email" placeholder="Электронная почта" value={email} onChange={(e) => setEmail(e.target.value)} required />
                            <input className={inputClass} type="tel" placeholder="Телефон" value={phone} onChange={(e) => setPhone(e.target.value)} required />
                            <button
                                type="submit"
                                className="bg-[#2b7fff] content-stretch flex flex-col items-center justify-center min-h-[56px] rounded-[16px] w-full mt-[12px] cursor-pointer transition-colors hover:bg-[#1f6fe8] active:bg-[#1a63d1]"
                            >
                                <span className="font-['Manrope:Medium',sans-serif] font-medium text-[16px] text-white leading-[1.35]">Отправить</span>
                            </button>
                        </form>
                    </>
                ) : (
                    <>
                        <div className="content-stretch flex flex-col gap-[8px] items-start pr-[40px]">
                            <p className="font-['Involve:SemiBold',sans-serif] leading-[1.15] not-italic text-[#34343c] text-[28px]">Заявка отправлена</p>
                            <p className="font-['Manrope:Medium',sans-serif] font-medium leading-[1.35] opacity-70 text-[#34343c] text-[16px]">Спасибо. Мы свяжемся с вами по указанным контактам.</p>
                        </div>
                        <button
                            onClick={onClose}
                            className="bg-[#f1f3f6] content-stretch flex flex-col items-center justify-center min-h-[56px] rounded-[16px] w-full cursor-pointer transition-colors hover:bg-[#e6e9ee] active:bg-[#dce0e6]"
                        >
                            <span className="font-['Manrope:Medium',sans-serif] font-medium text-[16px] text-[#34343c] leading-[1.35]">Закрыть</span>
                        </button>
                    </>
                )}
            </div>
        </div>
    );
}

export function ContactModal({ open, onClose }: { open: boolean; onClose: () => void }) {
    if (!open) return null;
    return <ContactModalForm onClose={onClose} />;
}
