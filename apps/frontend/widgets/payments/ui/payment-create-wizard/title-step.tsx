'use client';

import type { JSX } from 'react';
import { TextField } from '@/shared/ui/design';
import { WizardHeading } from './wizard-chrome';

/**
 * Шаг 2 визарда — название платежа (Figma 830:16721): необязательно,
 * дефолт — лейбл категории подставляется при сохранении.
 */

export type TitleStepProps = {
  readonly title: string;
  readonly onTitleChange: (title: string) => void;
};

export function TitleStep({ title, onTitleChange }: TitleStepProps): JSX.Element {
  return (
    <>
      <WizardHeading title="Дайте название платежу" />
      <div className="px-6 pt-4">
        <TextField
          variant="titleIn"
          title="Название"
          placeholder="Название платежа"
          description="Необязательно"
          value={title}
          onChange={(event) => onTitleChange(event.target.value)}
          onClear={() => onTitleChange('')}
        />
      </div>
    </>
  );
}
