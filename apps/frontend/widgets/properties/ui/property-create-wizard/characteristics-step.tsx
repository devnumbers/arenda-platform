'use client';

import { useEffect, useRef, useState } from 'react';
import type { ChangeEvent, JSX } from 'react';
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
import { isApartmentCategory, propertyHousingTypeOptions } from '@/features/properties';
import { Button, TextField } from '@/shared/ui/design';

/**
 * Шаг 3 «Характеристики» (#482, Figma 1218-54295): название объекта,
 * чип-группа «Тип жилья» для категории «Квартира» (закрывает тип
 * apartments) и поля из каталога по текущему типу — чипы enum (радиус
 * 16), числа с единицами-префиксами слева, этаж/этажность в две колонки,
 * описание в конце. Все характеристики необязательные
 * (docs/entities/obekt.md), готовность шага определяет название.
 *
 * Lossless при смене типа: в черновике чужие ключи хранятся, экран
 * рендерит только ключи каталога текущего типа (toWireAttributes);
 * скрытие заполненного — тихое, без нотиса (решение владельца).
 */

const NAME_MAX_LENGTH = 64;
const DESCRIPTION_MAX_LENGTH = 1024;

export type CharacteristicsStepProps = {
  readonly type: PropertyType;
  readonly onHousingTypeChange: (type: PropertyType) => void;
  readonly name: string;
  readonly onNameChange: (name: string) => void;
  readonly description: string;
  readonly onDescriptionChange: (description: string) => void;
  readonly attributes: PropertyAttributes;
  readonly onAttributesChange: (next: PropertyAttributes) => void;
};

export function CharacteristicsStep({
  type,
  onHousingTypeChange,
  name,
  onNameChange,
  description,
  onDescriptionChange,
  attributes,
  onAttributesChange,
}: CharacteristicsStepProps): JSX.Element {
  const nameInputRef = useRef<HTMLInputElement | null>(null);

  // Программный фокус на первое поле шага (паттерн шага «Адрес»).
  useEffect(() => {
    nameInputRef.current?.focus();
  }, []);

  // Ошибки характеристик показываются после правки поля (blur), набор
  // полей сменился — ошибки прошлого типа неактуальны. Подгонка состояния
  // при рендере — официальный паттерн React (как в AmountField).
  const [shownErrors, setShownErrors] = useState<{ type: PropertyType; errors: AttrErrors }>({
    type,
    errors: {},
  });
  if (shownErrors.type !== type) {
    setShownErrors({ type, errors: {} });
  }

  const wireAttributes = toWireAttributes(type, attributes);
  const fields = fieldsForType(type);

  // Живые ошибки: проводные значения по каталогу плюс «сырой» ввод,
  // который wire отбросил (например «5,5» в этаже) — без этого поле
  // пропало бы из payload молча (validateField даёт ошибку каталога).
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

  const showError = (key: AttrKey): string | undefined => shownErrors.errors[key];

  const updateAttribute = (key: AttrKey, value: string | number | undefined): void => {
    const next: Record<string, string | number> = { ...attributes };
    if (value === undefined) {
      delete next[key];
    } else {
      next[key] = value;
    }
    onAttributesChange(next);
  };

  // Сырой ввод фильтруется по типу поля (sanitizeAttributeInput: в числах
  // только цифры, разделитель и минус там, где каталог допускает) и
  // хранится строкой как набрано; нормализация в числа происходит в
  // проводных значениях при валидации и сабмите
  // (features/property-attributes/lib/wire).
  const handleAttributeInput = (field: AttrField, event: ChangeEvent<HTMLInputElement>): void => {
    const raw = sanitizeAttributeInput(field, event.currentTarget.value);
    updateAttribute(field.key, raw.length > 0 ? raw : undefined);
  };

  const handleFieldBlur = (): void => {
    setShownErrors({ type, errors: liveAttrErrors() });
  };

  const renderEnumField = (field: Extract<AttrField, { kind: 'enum' }>): JSX.Element => (
    <div key={field.key} className="flex flex-col gap-2">
      <span className="text-base font-medium leading-[18px] text-content">{fieldLabels[field.key]}</span>
      <div role="group" aria-label={fieldLabels[field.key]} className="flex flex-wrap gap-2">
        {field.options.map((option) => {
          const selected = attributes[field.key] === option;
          return (
            <Button
              key={option}
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

  const attributeValue = (key: AttrKey): string => {
    const raw = attributes[key];
    return raw === undefined ? '' : String(raw);
  };

  const renderInputField = (field: AttrField): JSX.Element => (
    <TextField
      key={field.key}
      title={fieldLabels[field.key]}
      value={attributeValue(field.key)}
      onChange={(event) => handleAttributeInput(field, event)}
      onBlur={handleFieldBlur}
      error={showError(field.key)}
      inputMode={field.kind === 'number' ? 'decimal' : field.kind === 'integer' ? 'numeric' : undefined}
      maxLength={field.kind === 'string' ? field.maxLen : undefined}
      prefix={field.kind === 'number' ? field.unit : undefined}
    />
  );

  // Поля каталога в порядке каталога; пара «Этаж» + «Этажность дома» —
  // в две колонки (Figma 1218:54295). Описание — отдельное поле
  // черновика, рендерится после полей каталога.
  const catalogUnits: JSX.Element[] = [];
  for (let index = 0; index < fields.length; index += 1) {
    const field = fields[index];
    if (field === undefined) {
      continue;
    }
    const nextField = fields[index + 1];
    if (field.key === 'floor' && nextField?.key === 'floors_total') {
      catalogUnits.push(
        <div key="floor-pair" className="grid grid-cols-2 gap-2">
          {renderInputField(field)}
          {renderInputField(nextField)}
        </div>,
      );
      index += 1;
      continue;
    }
    catalogUnits.push(
      field.kind === 'enum' ? renderEnumField(field) : renderInputField(field),
    );
  }

  return (
    <div className="flex flex-col gap-8 px-6 pt-6">
      <TextField
        ref={nameInputRef}
        title="Название объекта"
        placeholder="Моя квартира"
        maxLength={NAME_MAX_LENGTH}
        value={name}
        onChange={(event) => onNameChange(event.currentTarget.value)}
      />
      {isApartmentCategory(type) && (
        <div className="flex flex-col gap-2">
          <span className="text-base font-medium leading-[18px] text-content">Тип жилья</span>
          <div role="group" aria-label="Тип жилья" className="flex flex-wrap gap-2">
            {propertyHousingTypeOptions.map((option) => {
              const selected = type === option.value;
              return (
                <Button
                  key={option.value}
                  variant="secondary"
                  size="small"
                  className="rounded-button"
                  selected={selected}
                  aria-pressed={selected}
                  onClick={() => onHousingTypeChange(option.value)}
                >
                  {option.label}
                </Button>
              );
            })}
          </div>
        </div>
      )}
      {catalogUnits}
      <TextField
        title="Описание"
        multiline
        maxLength={DESCRIPTION_MAX_LENGTH}
        value={description}
        onChange={(event) => onDescriptionChange(event.currentTarget.value)}
      />
    </div>
  );
}
