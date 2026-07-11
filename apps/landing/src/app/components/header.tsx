import { useState, useEffect } from "react";
import { useNavigate, useLocation } from "react-router";
import { Menu, X } from "lucide-react";
import { LogoSmall } from "./logo";
import { Button } from "./button";
import { typo } from "./typo";

const NAV_ITEMS: { label: string; target: string }[] = [
  { label: "Возможности", target: "features" },
  { label: "Для кого", target: "audience" },
  { label: "Вопросы", target: "faq" },
  { label: "Поддержка", target: "support" },
];

function scrollToId(id: string) {
  const el = document.getElementById(id);
  if (el) el.scrollIntoView({ behavior: "smooth", block: "start" });
}

export function Header() {
  const [open, setOpen] = useState(false);
  const [scrolled, setScrolled] = useState(false);
  const navigate = useNavigate();
  const location = useLocation();

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 8);
    onScroll();
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  const goTo = (target: string) => {
    setOpen(false);
    if (location.pathname !== "/") {
      navigate("/#" + target);
    } else {
      scrollToId(target);
    }
  };

  const handleLogin = () => {
    setOpen(false);
    window.location.assign("/dashboard");
  };

  return (
    <div className="fixed left-0 right-0 top-0 z-50 flex flex-col items-center px-[16px] pt-[16px] min-[1201px]:px-[16px] max-[767px]:px-[12px] max-[767px]:pt-[12px]">
      <div
        className={`bg-white max-w-[1240px] w-full rounded-[24px] transition-shadow duration-300 ${
          scrolled ? "shadow-[0_8px_32px_rgba(0,0,0,0.08)]" : "shadow-none"
        }`}
      >
        <div className="flex items-center justify-between p-[12px] relative">
          {/* Logo */}
          <button onClick={() => goTo("top")} className="flex items-center pl-[7px] cursor-pointer shrink-0">
            <LogoSmall />
          </button>

          {/* Desktop nav */}
          <nav className="hidden min-[1201px]:flex gap-[32px] items-center">
            {NAV_ITEMS.map((item) => (
              <button
                key={item.target}
                onClick={() => goTo(item.target)}
                className="py-[10px] font-['Manrope:Medium',sans-serif] font-medium leading-[1.35] text-[#222] text-[16px] whitespace-nowrap cursor-pointer transition-opacity hover:opacity-60 active:opacity-40"
              >
                {typo(item.label)}
              </button>
            ))}
          </nav>

          {/* Right side */}
          <div className="flex gap-[4px] items-center justify-end">
            <Button variant="light-sm" onClick={handleLogin}>
              Войти
            </Button>
            {/* Burger (tablet + mobile) */}
            <button
              onClick={() => setOpen((v) => !v)}
              className="min-[1201px]:hidden bg-[#f1f3f6] flex items-center justify-center p-[10px] rounded-[12px] cursor-pointer transition-colors hover:bg-[#e7eaef] active:bg-[#dde1e8]"
              aria-label="Меню"
            >
              <span className="relative block size-[20px]">
                <Menu
                  size={20}
                  color="#222222"
                  className={`absolute inset-0 transition-all duration-300 ${
                    open ? "opacity-0 rotate-90 scale-75" : "opacity-100 rotate-0 scale-100"
                  }`}
                />
                <X
                  size={20}
                  color="#222222"
                  className={`absolute inset-0 transition-all duration-300 ${
                    open ? "opacity-100 rotate-0 scale-100" : "opacity-0 -rotate-90 scale-75"
                  }`}
                />
              </span>
            </button>
          </div>
        </div>

        {/* Mobile / tablet dropdown menu */}
        <div
          className="min-[1201px]:hidden grid transition-[grid-template-rows] duration-300 ease-out"
          style={{ gridTemplateRows: open ? "1fr" : "0fr" }}
        >
          <div className="overflow-hidden">
            <div
              className={`border-t border-[#f1f3f6] flex flex-col gap-[4px] px-[12px] py-[12px] transition-opacity duration-300 ${
                open ? "opacity-100" : "opacity-0"
              }`}
            >
              {NAV_ITEMS.map((item) => (
                <button
                  key={item.target}
                  onClick={() => goTo(item.target)}
                  className="text-left px-[16px] py-[12px] rounded-[12px] font-['Manrope:Medium',sans-serif] font-medium leading-[1.35] text-[#222] text-[16px] cursor-pointer transition-colors hover:bg-[#f1f3f6] active:bg-[#e7eaef]"
                >
                  {typo(item.label)}
                </button>
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
