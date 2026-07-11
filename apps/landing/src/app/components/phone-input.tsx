import { useLayoutEffect, useRef, useState } from "react";

// Formats up to 10 "national" digits into +7 (999) 000-00-00
export function formatPhone(national: string) {
  const n = national.slice(0, 10);
  let res = "+7";
  if (n.length === 0) return res + " ";
  res += " (" + n.slice(0, 3);
  if (n.length >= 3) res += ")";
  if (n.length > 3) res += " " + n.slice(3, 6);
  if (n.length > 6) res += "-" + n.slice(6, 8);
  if (n.length > 8) res += "-" + n.slice(8, 10);
  return res;
}

export function PhoneInput({
  value,
  onChange,
  className,
  placeholder,
}: {
  value: string; // national digits only (0-10)
  onChange: (national: string) => void;
  className?: string;
  placeholder?: string;
}) {
  const inputRef = useRef<HTMLInputElement>(null);
  const caretRef = useRef<number | null>(null);
  const [focused, setFocused] = useState(false);

  useLayoutEffect(() => {
    if (caretRef.current != null && inputRef.current) {
      const pos = caretRef.current;
      inputRef.current.setSelectionRange(pos, pos);
      caretRef.current = null;
    }
  });

  const display = value.length ? formatPhone(value) : focused ? "+7 " : "";

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const el = e.target;
    const raw = el.value;
    const caret = el.selectionStart ?? raw.length;
    const digitsBefore = raw.slice(0, caret).replace(/\D/g, "").length;

    let digits = raw.replace(/\D/g, "");
    let dropped = 0;
    if (digits.startsWith("8")) {
      digits = digits.slice(1);
      dropped = 1;
    } else if (digits.startsWith("7")) {
      digits = digits.slice(1);
      dropped = 1;
    }
    const national = digits.slice(0, 10);
    const formatted = national.length ? formatPhone(national) : "+7 ";
    const nBefore = Math.max(0, digitsBefore - dropped);

    // Map the national-digit index back to a caret position in the formatted string
    let pos: number;
    if (nBefore <= 0) {
      pos = Math.min(4, formatted.length);
    } else {
      let count = 0;
      pos = formatted.length;
      for (let i = 2; i < formatted.length; i++) {
        if (/\d/.test(formatted[i])) {
          count++;
          if (count === nBefore) {
            pos = i + 1;
            break;
          }
        }
      }
    }
    caretRef.current = pos;
    onChange(national);
  };

  const handleFocus = () => {
    setFocused(true);
    const next = value.length ? formatPhone(value) : "+7 ";
    caretRef.current = next.length;
  };

  return (
    <input
      ref={inputRef}
      type="tel"
      inputMode="tel"
      autoComplete="tel"
      required
      className={className}
      placeholder={placeholder}
      value={display}
      onChange={handleChange}
      onFocus={handleFocus}
      onBlur={() => setFocused(false)}
    />
  );
}
