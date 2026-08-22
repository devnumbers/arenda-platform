'use client';

import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type JSX,
  type KeyboardEvent,
  type ChangeEvent,
} from 'react';
import { useAddressSuggestions } from '@/features/properties';
import { useDebounce } from '@/shared/lib/hooks/useDebounce';
import { Button } from '@/shared/ui/button';
import { TextField } from '@/shared/ui/text-field';
import { runListboxAction } from '@/shared/ui/select/listbox-keyboard';
import styles from './PropertyAddressStep.module.css';

export type PropertyAddressStepProps = {
  value?: string;
  onChange: (address: string) => void;
  onNext: () => void;
  onBack?: () => void;
};

export function PropertyAddressStep({
  value,
  onChange,
  onNext,
}: PropertyAddressStepProps): JSX.Element {
  const [inputValue, setInputValue] = useState(value ?? '');
  const [isOpen, setIsOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState<number | null>(null);
  const debouncedQuery = useDebounce(inputValue, 300);
  const { data: suggestions, isLoading } = useAddressSuggestions(debouncedQuery);
  const wrapperRef = useRef<HTMLDivElement>(null);
  const listRef = useRef<HTMLUListElement>(null);
  const fieldRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    // Sync local input with the selected address when it is restored from
    // sessionStorage or changed by the parent wizard.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setInputValue(value ?? '');
  }, [value]);

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (
        wrapperRef.current &&
        !wrapperRef.current.contains(event.target as Node)
      ) {
        setIsOpen(false);
        setActiveIndex(null);
      }
    }

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const handleSelect = useCallback(
    (address: string) => {
      setInputValue(address);
      onChange(address);
      setIsOpen(false);
      setActiveIndex(null);
      // The dropdown unmounts on selection — hand focus back to the field.
      fieldRef.current?.focus();
    },
    [onChange],
  );

  const handleChange = (event: ChangeEvent<HTMLInputElement>) => {
    const nextValue = event.currentTarget.value;
    setInputValue(nextValue);
    setIsOpen(true);
    setActiveIndex(null);
    onChange(nextValue);
  };

  const handleFocus = () => {
    setIsOpen(true);
  };

  const handleKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (!suggestions || suggestions.length === 0) {
      if (event.key === 'Escape') {
        setIsOpen(false);
        setActiveIndex(null);
      }
      return;
    }

    if (event.key === 'ArrowDown') {
      event.preventDefault();
      setIsOpen(true);
      setActiveIndex((prev) =>
        prev === null ? 0 : Math.min(prev + 1, suggestions.length - 1),
      );
    } else if (event.key === 'ArrowUp') {
      event.preventDefault();
      setActiveIndex((prev) =>
        prev === null ? suggestions.length - 1 : Math.max(prev - 1, 0),
      );
    } else if (event.key === 'Enter' && activeIndex !== null) {
      event.preventDefault();
      const suggestion = suggestions[activeIndex];
      if (suggestion !== undefined) {
        handleSelect(suggestion.value);
      }
    } else if (event.key === 'Escape') {
      setIsOpen(false);
      setActiveIndex(null);
    }
  };

  // Roving-focus path for the options themselves (focus lands on an option
  // after a mouse click): Enter/Space pick, Escape closes back into the
  // field, arrows move focus and keep the field-driven active index in sync.
  const handleOptionKeyDown = (event: KeyboardEvent<HTMLLIElement>, index: number) => {
    const target = runListboxAction(event, {
      onSelect: () => {
        const suggestion = suggestions?.[index];
        if (suggestion) {
          handleSelect(suggestion.value);
        }
      },
      onClose: () => {
        setIsOpen(false);
        setActiveIndex(null);
        fieldRef.current?.focus();
      },
    });
    if (target && listRef.current) {
      const items = Array.from(listRef.current.querySelectorAll<HTMLElement>('[role="option"]'));
      const targetIndex = items.indexOf(target);
      if (targetIndex >= 0) {
        setActiveIndex(targetIndex);
      }
    }
  };

  const showDropdown =
    isOpen && debouncedQuery.trim().length >= 3;

  return (
    <div className={styles.root}>
      <h2 className={styles.heading}>Введите адрес</h2>

      <div className={styles.fieldWrapper} ref={wrapperRef}>
        <TextField
          label="Адрес"
          required
          fullWidth
          maxLength={500}
          value={inputValue}
          onChange={handleChange}
          onFocus={handleFocus}
          onKeyDown={handleKeyDown}
          placeholder=""
          ref={fieldRef}
        />

        {showDropdown && (
          <div className={styles.dropdown}>
            {isLoading ? (
              <div className={styles.message}>Загрузка...</div>
            ) : suggestions && suggestions.length > 0 ? (
              <ul
                className={styles.list}
                role="listbox"
                aria-label="Предложенные адреса"
                ref={listRef}
              >
                {suggestions.map((suggestion, index) => (
                  <li
                    key={suggestion.value}
                    className={styles.listItem}
                    role="option"
                    aria-selected={activeIndex === index}
                    data-active={activeIndex === index}
                    tabIndex={-1}
                    onClick={() => handleSelect(suggestion.value)}
                    onKeyDown={(event) => handleOptionKeyDown(event, index)}
                  >
                    {suggestion.value}
                  </li>
                ))}
              </ul>
            ) : (
              <div className={styles.message}>Адреса не найдены</div>
            )}
          </div>
        )}
      </div>

      <Button
        type="button"
        variant="primary"
        size="large"
        fullWidth
        disabled={!value}
        onClick={onNext}
        className={styles.continue}
      >
        Продолжить
      </Button>
    </div>
  );
}
