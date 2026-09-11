'use client';

import { type ChangeEvent, type JSX } from 'react';
import type { PropertyAttributes, PropertyType } from '@/entities/property';
import {
  enumLabels,
  fieldLabels,
  fieldsForType,
  sanitizeAttributeInput,
  toWireAttributes,
  validateAttributes,
  validateField,
  type AttrErrors,
  type AttrField,
  type AttrKey,
} from '@/features/property-attributes';
import { Button, TextField } from '@/shared/ui/design';

/**
 * Поля каталога характеристик по типу объекта — общая часть визарда
 * создания (шаг 3, Figma 1218:54295) и формы правки (карта #583, тикет
 * #590, Figma 1550:95852). Чипы enum (radius 16), поля с единицами-
 * префиксами слева; пары полей — в две колонки («Этаж» + «Этажность
 * дома», у квартиры также «Жилая площадь» + «Площадь кухни»).
 *
 * Значения — «сырые»: строки хранятся как набрано (фильтр
 * sanitizeAttributeInput), числа из объекта остаются числами;
 * нормализация в проводные значения — toWireAttributes на сабмите.
 *
 * Живые ошибки считаются на каждом рендере (проводные значения плюс
 * «сырой» ввод, который wire отбросил — без этого поле пропало бы из
 * payload молча), но показываются только при showErrors — гейт
 * вызывающего (после blur/попытки сабмита; смена типа сбрасывает гейт).
 * Ошибки сервера (fieldErrors PATCH) приходят в extraErrors и видны
 * всегда. Очистка поля — крестик (clearable, форма правки по макету
 * 1550:95852); визард макетом очистки не предусмотрен — не включает.
 */

export type PropertyCatalogFieldsProps = {
  readonly type: PropertyType;
  readonly attributes: PropertyAttributes;
  readonly onChange: (next: PropertyAttributes) => void;
  readonly showErrors: boolean;
  /** Правка поля ввода (blur) — сигнал вызывающему включить showErrors. */
  readonly onFieldsBlur?: () => void;
  readonly extraErrors?: AttrErrors;
  readonly clearable?: boolean;
};

/** Пары соседних полей, рисуемые в две колонки. */
function isPairedPair(current: AttrField, next: AttrField | undefined): boolean {
  if (next === undefined) {
    return false;
  }
  return (
    (current.key === 'floor' && next.key === 'floors_total')
    || (current.key === 'area_living' && next.key === 'area_kitchen')
  );
}

export function PropertyCatalogFields({
  type,
  attributes,
  onChange,
  showErrors,
  onFieldsBlur,
  extraErrors,
  clearable = false,
}: PropertyCatalogFieldsProps): JSX.Element {
  const fields = fieldsForType(type);

  const wireAttributes = toWireAttributes(type, attributes);

  // Живые ошибки: проводные значения по каталогу плюс «сырой» ввод,
  // который wire отбросил (например «5,5» в этаже).
  const liveAttrErrors = (): AttrErrors => {
    const errors: AttrErrors = { ...validateAttributes(type, wireAttributes) };
    const fieldKeys = new Set<string>(fields.map((field) => field.key));
    for (const [key, raw] of Object.entries(attributes)) {
      if (
        (typeof raw === 'string' || typeof raw === 'number')
        && fieldKeys.has(key)
        && wireAttributes[key] === undefined
      ) {
        const error = validateField(type, key as AttrKey, raw);
        if (error !== undefined) {
          errors[key as AttrKey] = error;
        }
      }
    }
    return errors;
  };

  const liveErrors = showErrors ? liveAttrErrors() : {};

  const showError = (key: AttrKey): string | undefined =>
    extraErrors?.[key] ?? liveErrors[key];

  const updateAttribute = (key: AttrKey, value: string | number | undefined): void => {
    const next: Record<string, string | number> = { ...attributes };
    if (value === undefined) {
      delete next[key];
    } else {
      next[key] = value;
    }
    onChange(next);
  };

  // Сырой ввод фильтруется по типу поля и хранится строкой как набран;
  // нормализация в числа происходит в проводных значениях при валидации
  // и сабмите (features/property-attributes/lib/wire).
  const handleAttributeInput = (field: AttrField, event: ChangeEvent<HTMLInputElement>): void => {
    const raw = sanitizeAttributeInput(field, event.currentTarget.value);
    updateAttribute(field.key, raw.length > 0 ? raw : undefined);
  };

  const attributeValue = (key: AttrKey): string => {
    const raw = attributes[key];
    return raw === undefined ? '' : String(raw);
  };

  const renderEnumField = (field: Extract<AttrField, { kind: 'enum' }>): JSX.Element => (
    <div key={field.key} className="flex flex-col gap-2">
      <span className="text-base font-medium leading-[18px] text-content">{fieldLabels[field.key]}</span>
      <div role="group" aria-label={fieldLabels[field.key]} className="flex flex-wrap gap-2">
        {field.options.map((option) => {
          const selected = attributeValue(field.key) === option;
          return (
            <Button
              key={option}
              // Форма правки оборачивает поля в <form>: у design-Button
              // нет дефолта type — без явного 'button' чип отправлял форму.
              type="button"
              variant="secondary"
              size="small"
              className="rounded-button"
              selected={selected}
              aria-pressed={selected}
              onClick={() => updateAttribute(field.key, selected ? undefined : option)}
            >
              {enumLabels[field.key]?.[option] ?? option}
            </Button>
          );
        })}
      </div>
      {showError(field.key) !== undefined && (
        <span className="text-[13px] leading-[15px] text-error">{showError(field.key)}</span>
      )}
    </div>
  );

  const renderInputField = (field: AttrField): JSX.Element => (
    <TextField
      key={field.key}
      title={fieldLabels[field.key]}
      value={attributeValue(field.key)}
      onChange={(event) => handleAttributeInput(field, event)}
      onBlur={onFieldsBlur}
      onClear={clearable ? () => updateAttribute(field.key, undefined) : undefined}
      error={showError(field.key)}
      inputMode={field.kind === 'number' ? 'decimal' : field.kind === 'integer' ? 'numeric' : undefined}
      maxLength={field.kind === 'string' ? field.maxLen : undefined}
      prefix={field.kind === 'number' ? field.unit : undefined}
    />
  );

  // Поля каталога в порядке каталога; пары полей — в две колонки.
  const catalogUnits: JSX.Element[] = [];
  for (let index = 0; index < fields.length; index += 1) {
    const field = fields[index];
    if (field === undefined) {
      continue;
    }
    if (field.kind !== 'enum' && isPairedPair(field, fields[index + 1])) {
      const nextField = fields[index + 1];
      if (nextField !== undefined) {
        catalogUnits.push(
          <div key={`${field.key}-pair`} className="grid grid-cols-2 gap-2">
            {renderInputField(field)}
            {renderInputField(nextField)}
          </div>,
        );
        index += 1;
        continue;
      }
    }
    catalogUnits.push(
      field.kind === 'enum' ? renderEnumField(field) : renderInputField(field),
    );
  }

  return <div className="flex flex-col gap-8">{catalogUnits}</div>;
}
