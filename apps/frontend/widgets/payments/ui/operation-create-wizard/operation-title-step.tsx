'use client';

import type { JSX } from 'react';
import { TextField } from '@/shared/ui/design';
import { WizardHeading } from '../payment-create-wizard/wizard-chrome';

/**
 * Шаг 2 визарда операции — название (Figma 1863:67296): «Что хотите
 * добавить?», поле «Название операции», необязательно, лимит 256 символов
 * со счётчиком; пустое поле замещается лейблом категории при сохранении
 * (канон buildPaymentCreateCommand).
 */

export type OperationTitleStepProps = {
  readonly title: string;
  readonly onTitleChange: (title: string) => void;
};

export function OperationTitleStep({
  title,
  onTitleChange,
}: OperationTitleStepProps): JSX.Element {
  return (
    <>
      <WizardHeading title="Что хотите добавить?" />
      <div className="px-6 pt-4">
        <TextField
          variant="titleIn"
          title="Название операции"
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
