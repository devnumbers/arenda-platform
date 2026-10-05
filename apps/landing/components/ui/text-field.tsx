"use client";

import { useId, useLayoutEffect, useRef } from "react";
import type { ComponentProps, JSX, ReactNode } from "react";
import { cn } from "@/lib/cn";
import { Cancel } from "./icons";
import { IconButton } from "./icon-button";

/** Поле ввода дизайн-слой (Figma 948:46646, «Input Field») — перенос из
 * apps/frontend (shared/ui/design/text-field.tsx). Бокс фиксированной
 * высоты 56px в обоих вариантах. Title Out — заголовок над боксом (тот
 * самый вариант из кадров модалки лендинга 3012-82067); Title In —
 * плавающий лейбл. Состояния: Error (красный бокс + текст ошибки), Limited
 * (счётчик красным), Disabled (opacity 0.5), Hover (inset-обводка 2px) —
 * фокус-кольца у поля нет намеренно (решение владельца 2026-08-26):
 * видимый признак фокуса — каретка. Кнопка очистки появляется при
 * непустом значении и переданном onClear; autoGrow — опциональное
 * авторасширение multiline. Отличие от исходника: иконка инлайн-SVG. */

export type TextFieldVariant = "titleOut" | "titleIn";

type TextFieldBaseProps = {
  readonly variant?: TextFieldVariant;
  readonly title?: string;
  readonly required?: boolean;
  readonly description?: string;
  readonly error?: string;
  readonly maxLength?: number;
  readonly onClear?: () => void;
  readonly suffix?: ReactNode;
  readonly prefix?: ReactNode;
  readonly autoGrow?: boolean;
};

export type TextFieldProps = TextFieldBaseProps &
  (
    | ({ readonly multiline?: false } & Omit<ComponentProps<"input">, "size" | "maxLength">)
    | ({ readonly multiline: true } & Omit<ComponentProps<"textarea">, "maxLength" | "ref">)
  );

export function TextField({
  className,
  variant = "titleOut",
  title,
  required = false,
  description,
  error,
  maxLength,
  onClear,
  suffix,
  prefix,
  multiline = false,
  autoGrow = false,
  value,
  disabled,
  placeholder,
  ...props
}: TextFieldProps): JSX.Element {
  const inputId = useId();
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);
  const titleNode =
    title !== undefined && required ? (
      <>
        {title} <span aria-hidden className="text-error">*</span>
      </>
    ) : (
      title
    );

  // Авторасширение multiline-поля: высота подгоняется под контент
  useLayoutEffect(() => {
    const el = textareaRef.current;
    if (!el || !autoGrow) return;
    el.style.height = "auto";
    el.style.height = `${el.scrollHeight}px`;
  }, [autoGrow, value]);
  const hasValue = typeof value === "string" && value.length > 0;
  const showClear = onClear !== undefined && hasValue && !disabled;
  const counter =
    maxLength !== undefined && typeof value === "string"
      ? `${value.length}/${maxLength}`
      : undefined;
  const counterDanger = maxLength !== undefined && typeof value === "string" && value.length >= maxLength;
  const bottomLeft = error ?? description;

  const box = cn(
    "flex w-full items-center rounded-button bg-surface-muted pl-[18px] pr-2 transition-shadow",
    multiline ? "min-h-14 flex-col justify-center py-[10px]" : "h-14 py-0",
    !disabled && error === undefined && "hover:shadow-[inset_0_0_0_2px_var(--dl-input-border)]",
    error !== undefined && "bg-surface-danger hover:shadow-none",
  );
  const input = cn(
    "w-full min-w-0 border-none bg-transparent text-base leading-[18px] text-content outline-none placeholder:text-content-secondary",
    multiline ? "resize-none" : "h-full",
  );

  return (
    <div className={cn("flex w-full flex-col gap-2 font-sans", disabled && "opacity-50", className)}>
      {variant === "titleOut" && titleNode !== undefined && (
        <label htmlFor={inputId} className="text-base font-medium leading-[18px] text-content">
          {titleNode}
        </label>
      )}
      <div className={box}>
        {variant === "titleIn" && !multiline && title !== undefined && (
          <div className="relative h-full min-w-0 flex-1">
            <input
              id={inputId}
              className="peer h-full w-full border-none bg-transparent pb-[11px] pt-[27px] text-base leading-[18px] text-content outline-none placeholder:text-transparent"
              disabled={disabled}
              value={value}
              placeholder={title}
              maxLength={maxLength}
              {...(props as ComponentProps<"input">)}
            />
            <label
              htmlFor={inputId}
              className={cn(
                "pointer-events-none absolute left-0 text-content-secondary transition-all duration-300",
                "top-[19px] text-base leading-[18px]",
                "peer-focus:top-[10px] peer-focus:text-[13px] peer-focus:leading-[15px]",
                "peer-[:not(:placeholder-shown)]:top-[10px] peer-[:not(:placeholder-shown)]:text-[13px] peer-[:not(:placeholder-shown)]:leading-[15px]",
              )}
            >
              {titleNode}
            </label>
          </div>
        )}
        {(variant === "titleOut" || title === undefined || multiline) && (
          <>
            {prefix !== undefined && (
              <span aria-hidden className="pr-1 text-base leading-[18px] text-content-secondary">
                {prefix}
              </span>
            )}
            {multiline ? (
              <textarea
                id={inputId}
                ref={textareaRef}
                className={cn(input, "min-h-[54px]", autoGrow && "max-h-[200px] overflow-y-auto")}
                disabled={disabled}
                value={value}
                placeholder={placeholder}
                maxLength={maxLength}
                rows={3}
                {...(props as ComponentProps<"textarea">)}
              />
            ) : (
              <input
                id={inputId}
                className={cn(input, prefix !== undefined && "px-0")}
                disabled={disabled}
                value={value}
                placeholder={placeholder}
                maxLength={maxLength}
                {...(props as ComponentProps<"input">)}
              />
            )}
            {suffix !== undefined && (
              <span aria-hidden className="pl-1 text-base leading-[18px] text-content-secondary">
                {suffix}
              </span>
            )}
          </>
        )}
        {showClear && (
          <IconButton
            icon={<Cancel />}
            label="Очистить поле"
            variant={error !== undefined ? "danger" : "secondary"}
            disabled={disabled}
            onClick={onClear}
          />
        )}
      </div>
      {(bottomLeft !== undefined || counter !== undefined) && (
        <div className="flex items-center justify-between gap-2 text-[13px] leading-[15px]">
          {bottomLeft !== undefined && (
            <span className={cn(error !== undefined ? "text-error" : "text-content-tertiary")}>
              {bottomLeft}
            </span>
          )}
          {counter !== undefined && (
            <span className={cn(counterDanger ? "text-error" : "text-content-tertiary")}>{counter}</span>
          )}
        </div>
      )}
    </div>
  );
}
