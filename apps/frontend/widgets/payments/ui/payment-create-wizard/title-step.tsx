'use client';

import type { JSX } from 'react';
import { TextField } from '@/shared/ui/design';
import { WizardHeading } from './wizard-chrome';

/**
 * Шаг 2 визарда — название платежа (Figma 1049:47385): необязательно,
 * лимит 256 символов со счётчиком; дефолт — лейбл категории подставляется
 * при сохранении.
 */

export type TitleStepProps = {
  readonly title: string;
  readonly onTitleChange: (title: string) => void;
};

export function TitleStep({ title, onTitleChange }: TitleStepProps): JSX.Element {
  return (
    <>
      <WizardHeading title="Назовите платеж" />
      <div className="px-6 pt-4">
        <TextField
          variant="titleIn"
          title="Название платежа"
          description="Необязательно"
          maxLength={256}
          value={title}
          onChange={(event) => onTitleChange(event.target.value)}
          onClear={() => onTitleChange('')}
        />
      </div>
    </>
  );
}
