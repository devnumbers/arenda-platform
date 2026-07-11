import { Link } from "react-router";
import { LogoLarge } from "./logo";
import { Button } from "./button";
import { TelegramIcon, LetterIcon, CommentIcon } from "./icons";
import { typo } from "./typo";

export function Footer({
  onContact,
  onCopyEmail,
}: {
  onContact: () => void;
  onCopyEmail: () => void;
}) {
  return (
    <div id="support" className="scroll-mt-[110px] w-full flex flex-col items-center p-[80px] max-[767px]:pt-[96px] max-[767px]:pb-[20px] max-[767px]:px-[20px]">
      <div className="flex flex-col gap-[64px] max-[767px]:gap-[32px] items-start max-[767px]:items-center max-w-[1200px] w-full">
        <LogoLarge className="h-[64px] w-[252px] max-[767px]:h-[32px] max-[767px]:w-[126px]" />

        {/* Contact buttons */}
        <div className="flex gap-[12px] items-center max-[767px]:flex-col max-[767px]:gap-[8px] max-[767px]:w-full">
          <Button variant="light" icon={<TelegramIcon />} className="max-[767px]:w-full">
            {typo("Написать в Telegram")}
          </Button>
          <Button variant="light" icon={<LetterIcon />} onClick={onCopyEmail} className="max-[767px]:w-full">
            hello@rentlee.ru
          </Button>
          <Button variant="light" icon={<CommentIcon />} onClick={onContact} className="max-[767px]:w-full">
            {typo("Связаться с нами")}
          </Button>
        </div>

        {/* Legal + copyright */}
        <div className="flex gap-[32px] items-start w-full max-[767px]:flex-col max-[767px]:gap-[8px] font-['Manrope:Medium',sans-serif] font-medium leading-[1.35] text-[#222] text-[16px] whitespace-nowrap">
          <span className="opacity-40 max-[767px]:order-3">© 2026 Рентли</span>
          <Link to="/privacy" className="max-[767px]:order-1 transition-opacity hover:opacity-60 active:opacity-40">
            {typo("Политика конфиденциальности")}
          </Link>
          <Link to="/terms" className="max-[767px]:order-2 transition-opacity hover:opacity-60 active:opacity-40">
            {typo("Пользовательское соглашение")}
          </Link>
        </div>
      </div>
    </div>
  );
}
