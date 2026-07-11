import svgPaths from "../../imports/19201201/svg-wnwv43114b";

// Small logo used in the header (viewBox 110 x 28)
export function LogoSmall({ className }: { className?: string }) {
  return (
    <div className={className ?? "h-[28px] w-[110px]"}>
      <svg className="block size-full" fill="none" preserveAspectRatio="xMidYMid meet" viewBox="0 0 110 28">
        <g>
          <path d={svgPaths.p3a668a00} fill="#BCDCFF" />
          <path d={svgPaths.p3bc90b00} fill="white" />
          <path d={svgPaths.p36b4f230} fill="#2B7FFF" />
          <path d={svgPaths.p31f0d2f0} fill="#2B7FFF" />
          <path d={svgPaths.p1e3dc000} fill="#2B7FFF" />
          <path d={svgPaths.p37a5ed00} fill="#2B7FFF" />
          <path d={svgPaths.p322cf000} fill="#2B7FFF" />
          <path d={svgPaths.p29905d40} fill="#2B7FFF" />
          <path d={svgPaths.p17ff0300} fill="#2B7FFF" />
        </g>
      </svg>
    </div>
  );
}

// Large logo used in the footer (viewBox 252 x 64)
export function LogoLarge({ className }: { className?: string }) {
  return (
    <div className={className ?? "h-[64px] w-[252px]"}>
      <svg className="block size-full" fill="none" preserveAspectRatio="xMidYMid meet" viewBox="0 0 252 64">
        <g>
          <path d={svgPaths.p209aa180} fill="#BCDCFF" />
          <path d={svgPaths.p38438a00} fill="white" />
          <path d={svgPaths.p92f8700} fill="#2B7FFF" />
          <path d={svgPaths.p1d147780} fill="#2B7FFF" />
          <path d={svgPaths.p32b3f300} fill="#2B7FFF" />
          <path d={svgPaths.p1610a700} fill="#2B7FFF" />
          <path d={svgPaths.p2bebfd00} fill="#2B7FFF" />
          <path d={svgPaths.p28c0280} fill="#2B7FFF" />
          <path d={svgPaths.p2caae100} fill="#2B7FFF" />
        </g>
      </svg>
    </div>
  );
}
