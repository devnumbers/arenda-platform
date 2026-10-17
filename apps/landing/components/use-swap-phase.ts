"use client";

import { useEffect, useRef, useState } from "react";

// Общая машина подмены текста (текст шага в «Как начать пользоваться»,
// цена в «Тарифах» при смене Год/Месяц): по swap(next) — уход
// (leaving, outMs), затем подмена значения и въезд (entering), по
// окончании въезд-анимации (settle из onAnimationEnd) — обратно в idle.
// Guard-значение (повторный клик по активному), имя подменяемого
// значения и классы фаз остаются у секции — машина ведёт только фазу
// и подменяемое значение.
export type SwapPhase = "idle" | "leaving" | "entering";

export function useSwapPhase<Value>({
  initial,
  outMs,
}: {
  initial: Value;
  outMs: number;
}) {
  const [value, setValue] = useState<Value>(initial);
  const [phase, setPhase] = useState<SwapPhase>("idle");
  const swapTimer = useRef<number | null>(null);

  useEffect(() => {
    return () => {
      if (swapTimer.current !== null) {
        window.clearTimeout(swapTimer.current);
      }
    };
  }, []);

  const swap = (next: Value) => {
    if (swapTimer.current !== null) {
      window.clearTimeout(swapTimer.current);
      swapTimer.current = null;
    }
    if (phase === "entering" || phase === "leaving") {
      // Клик посреди въезда или ухода: значение подменяем сразу и
      // переигрываем въезд — фазу ухода второй раз не гоняем (при спаме
      // кликов текст не пустеет дольше одного короткого ухода).
      setValue(next);
      setPhase("entering");
      return;
    }
    setPhase("leaving");
    swapTimer.current = window.setTimeout(() => {
      setValue(next);
      setPhase("entering");
    }, outMs);
  };

  // Конец въезд-анимации (onAnimationEnd подменяемого узла) — машина
  // возвращается в idle.
  const settle = () => {
    setPhase("idle");
  };

  return { phase, value, swap, settle };
}
