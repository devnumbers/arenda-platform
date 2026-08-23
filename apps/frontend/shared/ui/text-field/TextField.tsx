'use client';

import {
  forwardRef,
  useId,
  useState,
  type ChangeEvent,
  type InputHTMLAttributes,
  type JSX,
  type ReactNode,
  type TextareaHTMLAttributes,
} from 'react';
import clsx from 'clsx';
import { Icon } from '@/shared/ui/icon';
import styles from './TextField.module.css';

export type TextFieldLabelPlacement = 'inside' | 'outside';

export type TextFieldProps = {
  readonly label?: string;
  readonly labelPlacement?: TextFieldLabelPlacement;
  readonly required?: boolean;
  readonly helperText?: ReactNode;
  readonly error?: ReactNode;
  readonly icon?: ReactNode;
  readonly fullWidth?: boolean;
  readonly multiline?: boolean;
  readonly maxLength?: number;
  readonly showCounter?: boolean;
  readonly className?: string;
} & (
  | (InputHTMLAttributes<HTMLInputElement> & { readonly multiline?: false })
  | (TextareaHTMLAttributes<HTMLTextAreaElement> & { readonly multiline: true })
);

export const TextField = forwardRef<HTMLInputElement | HTMLTextAreaElement, TextFieldProps>(
  function TextField(props, ref): JSX.Element {
    const {
      label,
      labelPlacement = 'outside',
      required,
      helperText,
      error,
      icon,
      fullWidth,
      multiline,
      maxLength,
      showCounter,
      className,
      ...rest
    } = props;

    const fieldId = useId();
    const inputId = rest.id ?? `text-field-${fieldId}`;
    const hasError = Boolean(error);
    const isDisabled = Boolean(rest.disabled);

    const placeholderProp = (rest as InputHTMLAttributes<HTMLInputElement>).placeholder;
    const inputPlaceholder =
      labelPlacement === 'inside' && label ? '\u00A0' : placeholderProp;

    const [internalValue, setInternalValue] = useState('');
    const currentValue =
      rest.value !== undefined ? String(rest.value) : internalValue;
    const currentLength = currentValue.length;
    const isExceeded = maxLength !== undefined && currentLength > maxLength;

    const handleChange = (event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
      setInternalValue(event.currentTarget.value);
      if (multiline) {
        (rest as TextareaHTMLAttributes<HTMLTextAreaElement>).onChange?.(
          event as ChangeEvent<HTMLTextAreaElement>
        );
      } else {
        (rest as InputHTMLAttributes<HTMLInputElement>).onChange?.(
          event as ChangeEvent<HTMLInputElement>
        );
      }
    };

    const labelContent = label ? (
      <label htmlFor={inputId} className={styles.label}>
        {label}
        {required && <span className={styles.required}>*</span>}
      </label>
    ) : null;

    const floatingLabelContent =
      labelPlacement === 'inside' && label ? (
        <label htmlFor={inputId} className={styles.floatingLabel}>
          {label}
          {required && <span className={styles.required}>*</span>}
        </label>
      ) : null;

    const hintId = Boolean(helperText) || Boolean(error) ? `${inputId}-hint` : undefined;
    const counterId =
      maxLength !== undefined && showCounter !== false ? `${inputId}-counter` : undefined;
    const describedBy = [hintId, counterId].filter(Boolean).join(' ') || undefined;

    const fieldClassName = clsx(
      styles.field,
      icon && styles.withIcon,
      hasError && styles.error,
      isDisabled && styles.disabled,
      multiline && styles.multiline
    );

    const controlClassName = multiline ? styles.textarea : styles.input;

    return (
      <div className={clsx(styles.root, fullWidth && styles.fullWidth, className)}>
        {labelPlacement === 'outside' && labelContent}
        <div className={fieldClassName}>
          {icon && (
            <span className={styles.icon}>
              <Icon size="m">{icon}</Icon>
            </span>
          )}
          {multiline ? (
            <textarea
              {...(rest as TextareaHTMLAttributes<HTMLTextAreaElement>)}
              id={inputId}
              ref={ref as React.Ref<HTMLTextAreaElement>}
              className={controlClassName}
              aria-invalid={hasError || undefined}
              aria-describedby={describedBy}
              maxLength={maxLength}
              placeholder={inputPlaceholder}
              onChange={handleChange}
            />
          ) : (
            <input
              {...(rest as InputHTMLAttributes<HTMLInputElement>)}
              id={inputId}
              ref={ref as React.Ref<HTMLInputElement>}
              className={controlClassName}
              aria-invalid={hasError || undefined}
              aria-describedby={describedBy}
              maxLength={maxLength}
              placeholder={inputPlaceholder}
              onChange={handleChange}
            />
          )}
          {floatingLabelContent}
        </div>
        {(Boolean(helperText) || Boolean(error) || (maxLength !== undefined && showCounter !== false)) && (
          <div className={styles.footer}>
            {(Boolean(helperText) || Boolean(error)) && (
              <span id={hintId} className={clsx(styles.hint, hasError && styles.hintError)}>
                {error ?? helperText}
              </span>
            )}
            {maxLength !== undefined && showCounter !== false && (
              <span id={counterId} className={clsx(styles.counter, isExceeded && styles.counterExceeded)}>
                {currentLength}/{maxLength}
              </span>
            )}
          </div>
        )}
      </div>
    );
  }
);
