'use client';

import { type JSX, type ChangeEvent, useCallback } from 'react';
import { Button, TextField } from '@/shared/ui/design';
import type { PropertyType, PropertyAttributes } from '@/entities/property';
import {
  fieldsForType,
  groupLabels,
  fieldLabels,
  enumLabels,
  type AttrField,
} from '@/features/property-attributes';
import type { AttrKey, AttrErrors } from '@/features/property-attributes';
import styles from './PropertyAttributesFields.module.css';

export type PropertyAttributesFieldsProps = {
  readonly type: PropertyType;
  readonly value: PropertyAttributes;
  readonly onChange: (next: PropertyAttributes) => void;
  readonly errors?: AttrErrors;
  readonly onFieldBlur?: () => void;
};

export function PropertyAttributesFields({
  type,
  value,
  onChange,
  errors,
  onFieldBlur,
}: PropertyAttributesFieldsProps): JSX.Element {
  const fields = fieldsForType(type);

  const updateAttributes = useCallback(
    (key: AttrKey, val: string | number | undefined) => {
      const next: Record<string, string | number> = { ...value };
      if (val === undefined) {
        delete next[key];
      } else {
        next[key] = val;
      }
      onChange(next);
    },
    [value, onChange],
  );

  const handleEnumChange = useCallback(
    (key: AttrKey, option: string, selected: boolean) => {
      updateAttributes(key, selected ? undefined : option);
    },
    [updateAttributes],
  );

  const handleNumberChange = useCallback(
    (key: AttrKey, raw: string) => {
      if (raw === '') {
        updateAttributes(key, undefined);
        return;
      }
      const num = Number(raw);
      if (Number.isFinite(num)) {
        updateAttributes(key, num);
      } else {
        updateAttributes(key, raw);
      }
    },
    [updateAttributes],
  );

  const handleStringChange = useCallback(
    (key: AttrKey, raw: string) => {
      if (raw === '') {
        updateAttributes(key, undefined);
        return;
      }
      updateAttributes(key, raw);
    },
    [updateAttributes],
  );

  const renderField = (field: AttrField) => {
    if (field.kind === 'enum') {
      const selectedValue = String(value[field.key] ?? '');
      return (
        <div className={styles.field} key={field.key}>
          <span className={styles.fieldLabel}>{fieldLabels[field.key]}</span>
          {/* Канонные Button-чипы (легаси снесён, #901): selected
              подсвечивает Primary — прямая семантика канонного варианта;
              aria-pressed остаётся на кнопке. */}
          <div className={styles.chips} role="group" aria-label={fieldLabels[field.key]}>
            {field.options.map((option) => {
              const selected = selectedValue === option;
              return (
                <Button
                  key={option}
                  type="button"
                  variant={selected ? 'primary' : 'secondary'}
                  size="small"
                  aria-pressed={selected}
                  onClick={() => handleEnumChange(field.key, option, selected)}
                >
                  {enumLabels[field.key]?.[option] ?? option}
                </Button>
              );
            })}
          </div>
          {errors?.[field.key] && (
            <span className={styles.fieldError}>{errors[field.key]}</span>
          )}
        </div>
      );
    }

    if (field.kind === 'number') {
      return (
        <div className={styles.field} key={field.key}>
          <div className={styles.labeledInput}>
            <TextField
              title={fieldLabels[field.key]}
              type="number"
              inputMode="decimal"
              step={field.decimals > 0 ? 'any' : '1'}
              value={String(value[field.key] ?? '')}
              error={errors?.[field.key]}
              onChange={(e: ChangeEvent<HTMLInputElement>) =>
                handleNumberChange(field.key, e.currentTarget.value)
              }
              onBlur={() => onFieldBlur?.()}
            />
            <span className={styles.unit} aria-hidden="true">{field.unit}</span>
          </div>
        </div>
      );
    }

    if (field.kind === 'integer') {
      return (
        <div className={styles.field} key={field.key}>
          <TextField
            title={fieldLabels[field.key]}
            type="number"
            inputMode="numeric"
            step="1"
            value={String(value[field.key] ?? '')}
            error={errors?.[field.key]}
            onChange={(e: ChangeEvent<HTMLInputElement>) =>
              handleNumberChange(field.key, e.currentTarget.value)
            }
            onBlur={() => onFieldBlur?.()}
          />
        </div>
      );
    }

    // field.kind === 'string'
    return (
      <div className={styles.field} key={field.key}>
        <TextField
          title={fieldLabels[field.key]}
          maxLength={field.maxLen}
          value={String(value[field.key] ?? '')}
          error={errors?.[field.key]}
          onChange={(e: ChangeEvent<HTMLInputElement>) =>
            handleStringChange(field.key, e.currentTarget.value)
          }
          onBlur={() => onFieldBlur?.()}
        />
      </div>
    );
  };

  // Precompute group headers immutably: a header is shown before a field when
  // the field's group is non-null and differs from the previous field's group.
  const renderedFields = fields.map((field, index) => {
    const prevGroup = index === 0 ? null : (fields[index - 1]?.group ?? null);
    const showHeader = field.group !== null && field.group !== prevGroup;
    return { field, showHeader };
  });

  return (
    <div className={styles.root}>
      {renderedFields.map(({field, showHeader}) => (
        <div key={field.key}>
          {showHeader && field.group !== null && (
            <h3 className={styles.groupTitle}>{groupLabels[field.group]}</h3>
          )}
          {renderField(field)}
        </div>
      ))}
    </div>
  );
}
