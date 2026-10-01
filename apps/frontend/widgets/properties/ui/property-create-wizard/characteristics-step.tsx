'use client';

import { useEffect, useRef, useState } from 'react';
import type { JSX } from 'react';
import type { PropertyAttributes, PropertyType } from '@/entities/property';
import { propertyAutonamePhrase } from '@/features/properties';
import { TextField } from '@/shared/ui/design';
import { PropertyCatalogFields } from '../property-fields/property-catalog-fields';
import { PropertyHousingTypeChips } from '../property-fields/property-housing-type-chips';

/**
 * Шаг 3 «Характеристики» (#482, Figma 1218-54295): название объекта,
 * чип-группа «Тип жилья» для категории «Квартира» (закрывает тип
 * apartments) и поля из каталога по текущему типу, описание в конце.
 * Все характеристики необязательные (docs/entities/obekt.md),
 * готовность шага определяет название. Поля каталога — общая часть с
 * формой правки (property-fields, тикет #590).
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
  readonly onAttributesChange: (attributes: PropertyAttributes) => void;
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
  const [shownErrorsType, setShownErrorsType] = useState<{ type: PropertyType; shown: boolean }>({
    type,
    shown: false,
  });
  if (shownErrorsType.type !== type) {
    setShownErrorsType({ type, shown: false });
  }

  return (
    <div className="flex flex-col gap-8 px-6 pt-6">
      <TextField
        ref={nameInputRef}
        title="Название объекта"
        placeholder={propertyAutonamePhrase(type)}
        maxLength={NAME_MAX_LENGTH}
        value={name}
        onChange={(event) => onNameChange(event.currentTarget.value)}
      />
      <PropertyHousingTypeChips type={type} onChange={onHousingTypeChange} />
      <PropertyCatalogFields
        type={type}
        attributes={attributes}
        onChange={onAttributesChange}
        showErrors={shownErrorsType.type === type && shownErrorsType.shown}
        onFieldsBlur={() => setShownErrorsType({ type, shown: true })}
      />
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
