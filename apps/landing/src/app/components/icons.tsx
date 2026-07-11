import svgPaths from "../../imports/19201201/svg-wnwv43114b";

export function UserIcon() {
  return (
    <div className="relative shrink-0 size-[40px]">
      <svg className="block size-full" fill="none" preserveAspectRatio="none" viewBox="0 0 40 40">
        <g>
          <path d={svgPaths.p19c72500} stroke="white" strokeWidth="3" />
          <path d={svgPaths.p1270c200} stroke="white" strokeWidth="3" />
        </g>
      </svg>
    </div>
  );
}

export function CaseIcon() {
  return (
    <div className="relative shrink-0 size-[40px]">
      <svg className="block size-full" fill="none" preserveAspectRatio="none" viewBox="0 0 40 40">
        <path d={svgPaths.p38d5b00} stroke="#222222" strokeWidth="3" />
      </svg>
    </div>
  );
}

export function BuildingIcon() {
  return (
    <div className="relative shrink-0 size-[40px]">
      <svg className="block size-full" fill="none" preserveAspectRatio="none" viewBox="0 0 40 40">
        <path d={svgPaths.p26ec6500} fill="white" />
      </svg>
    </div>
  );
}

export function PlusIcon({ open }: { open?: boolean }) {
  // "+" turns into "−" when the FAQ item is open (collapse the vertical stroke)
  return (
    <div className="relative shrink-0 size-[24px]">
      <svg className="block size-full" fill="none" preserveAspectRatio="none" viewBox="0 0 24 24">
        <g>
          <path d="M4 12H20" stroke="#222222" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5" />
          <path
            d="M12 4V20"
            stroke="#222222"
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth="1.5"
            className="origin-center transition-transform duration-200"
            style={{ transform: open ? "scaleY(0)" : "scaleY(1)" }}
          />
        </g>
      </svg>
    </div>
  );
}

export function TelegramIcon() {
  return (
    <div className="relative shrink-0 size-[22px]">
      <svg className="block size-full" fill="none" preserveAspectRatio="none" viewBox="0 0 22 22">
        <path d={svgPaths.p26fbfe00} fill="#2B7FFF" />
      </svg>
    </div>
  );
}

export function LetterIcon() {
  return (
    <div className="relative shrink-0 size-[22px]">
      <svg className="block size-full" fill="none" preserveAspectRatio="none" viewBox="0 0 22 22">
        <path clipRule="evenodd" d={svgPaths.p134f6a00} fill="#2B7FFF" fillRule="evenodd" />
      </svg>
    </div>
  );
}

export function CommentIcon() {
  return (
    <div className="relative shrink-0 size-[22px]">
      <svg className="block size-full" fill="none" preserveAspectRatio="none" viewBox="0 0 22 22">
        <path clipRule="evenodd" d={svgPaths.pfae0300} fill="#2B7FFF" fillRule="evenodd" />
      </svg>
    </div>
  );
}
