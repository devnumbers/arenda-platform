import { Check } from "lucide-react";

export function Toast({ visible, message }: { visible: boolean; message: string }) {
  return (
    <div
      className={`fixed left-1/2 -translate-x-1/2 top-[96px] max-[767px]:top-[84px] z-[90] transition-all duration-300 ${
        visible ? "opacity-100 translate-y-0" : "opacity-0 -translate-y-2 pointer-events-none"
      }`}
    >
      <div className="bg-white rounded-[16px] shadow-[0_8px_32px_rgba(0,0,0,0.12)] flex gap-[8px] items-center px-[20px] py-[14px]">
        <span className="bg-[#2b7fff] flex items-center justify-center rounded-[8px] size-[24px] shrink-0">
          <Check size={16} color="white" strokeWidth={3} />
        </span>
        <span className="font-['Manrope:Medium',sans-serif] font-medium leading-[1.35] text-[#222] text-[16px] whitespace-nowrap">
          {message}
        </span>
      </div>
    </div>
  );
}
