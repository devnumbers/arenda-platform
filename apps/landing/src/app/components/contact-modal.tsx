import { useState } from "react";
import { Link } from "react-router";
import { X, Check } from "lucide-react";
import { Button } from "./button";
import { PhoneInput } from "./phone-input";
import { typo } from "./typo";

export function ContactModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [phone, setPhone] = useState("");
  const [message, setMessage] = useState("");
  const [agree, setAgree] = useState(false);
  const [sent, setSent] = useState(false);

  if (!open) return null;

  const canSubmit = phone.length === 10 && agree;

  const reset = () => {
    setName("");
    setEmail("");
    setPhone("");
    setMessage("");
    setAgree(false);
    setSent(false);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!canSubmit) return;
    setSent(true);
  };

  const inputClass =
    "bg-[#f1f3f6] rounded-[16px] px-[20px] py-[16px] w-full font-['Manrope:Medium',sans-serif] font-medium text-[16px] text-[#222] leading-[1.35] placeholder:text-[#222] placeholder:opacity-40 outline-none focus:ring-2 focus:ring-[#2b7fff]/30";

  return (
    <div
      className="fixed inset-0 z-[100] flex items-center justify-center p-[16px] bg-black/40"
      onClick={handleClose}
    >
      <div
        className="bg-white rounded-[32px] w-full max-w-[520px] max-h-[90vh] overflow-y-auto p-[40px] max-[767px]:p-[24px] relative"
        onClick={(e) => e.stopPropagation()}
      >
        <button
          onClick={handleClose}
          className="absolute right-[24px] top-[24px] flex items-center justify-center p-[8px] rounded-[12px] cursor-pointer transition-colors hover:bg-[#f1f3f6] active:bg-[#e7eaef]"
          aria-label="Закрыть"
        >
          <X size={24} color="#222222" />
        </button>

        {!sent ? (
          <form onSubmit={handleSubmit} className="flex flex-col gap-[24px]">
            <div className="flex flex-col gap-[8px] pr-[40px]">
              <p className="font-['Involve:SemiBold',sans-serif] leading-[1.15] text-[#222] text-[32px] max-[767px]:text-[24px]">
                {typo("Оставьте заявку")}
              </p>
            </div>

            <div className="flex flex-col gap-[8px]">
              <input
                className={inputClass}
                placeholder={typo("Имя")}
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
              <input
                className={inputClass}
                type="email"
                placeholder={typo("Электронная почта")}
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
              <PhoneInput className={inputClass} placeholder={typo("Телефон")} value={phone} onChange={setPhone} />
              <textarea
                className={`${inputClass} resize-none min-h-[100px]`}
                placeholder={typo("Сообщение")}
                rows={4}
                value={message}
                onChange={(e) => setMessage(e.target.value)}
              />
            </div>

            <label className="flex gap-[12px] items-start cursor-pointer">
              <input
                type="checkbox"
                checked={agree}
                onChange={(e) => setAgree(e.target.checked)}
                className="sr-only peer"
              />
              <span
                className={`mt-[1px] size-[24px] shrink-0 rounded-[8px] flex items-center justify-center transition-colors ${
                  agree ? "bg-[#2b7fff]" : "bg-[#f1f3f6]"
                }`}
              >
                {agree && <Check size={16} color="white" strokeWidth={3} />}
              </span>
              <span className="font-['Manrope:Medium',sans-serif] font-medium leading-[1.35] text-[#222] text-[14px] opacity-70">
                {"Я соглашаюсь с "}
                <Link to="/privacy" className="text-[#2b7fff] no-underline">
                  {typo("Политикой конфиденциальности")}
                </Link>{" "}
                {"и "}
                <Link to="/terms" className="text-[#2b7fff] no-underline">
                  {typo("Пользовательским соглашением")}
                </Link>
              </span>
            </label>

            <Button type="submit" variant="primary" disabled={!canSubmit} className="w-full">
              Отправить
            </Button>
          </form>
        ) : (
          <div className="flex flex-col gap-[24px] items-start">
            <div className="flex flex-col gap-[8px] pr-[40px]">
              <p className="font-['Involve:SemiBold',sans-serif] leading-[1.15] text-[#222] text-[32px] max-[767px]:text-[24px]">
                Заявка отправлена
              </p>
              <p className="font-['Manrope:Medium',sans-serif] font-medium leading-[1.35] opacity-70 text-[#222] text-[16px]">
                Спасибо. Мы свяжемся с вами в ближайшее время
              </p>
            </div>
            <Button variant="primary" onClick={handleClose} className="w-full">
              Закрыть
            </Button>
          </div>
        )}
      </div>
    </div>
  );
}
