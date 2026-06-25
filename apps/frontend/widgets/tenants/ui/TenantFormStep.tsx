'use client';

import type { JSX } from 'react';
import { Button } from '@/shared/ui/button';
import { TextField } from '@/shared/ui/text-field';
import styles from './TenantFormStep.module.css';

export type TenantFormStepProps = {
  readonly name: string;
  readonly surname: string;
  readonly patronymic: string;
  readonly phone: string;
  readonly comment: string;
  readonly isLoading: boolean;
  readonly error?: string;
  readonly onNameChange: (value: string) => void;
  readonly onSurnameChange: (value: string) => void;
  readonly onPatronymicChange: (value: string) => void;
  readonly onPhoneChange: (value: string) => void;
  readonly onCommentChange: (value: string) => void;
  readonly onSubmit: () => void;
};

const MAX_COMMENT_LENGTH = 500;

function isPhoneValid(phone: string): boolean {
  if (phone.trim() === '') return true;
  const digits = phone.replace(/\D/g, '');
  return digits.length >= 10;
}

export function TenantFormStep({
  name,
  surname,
  patronymic,
  phone,
  comment,
  isLoading,
  error,
  onNameChange,
  onSurnameChange,
  onPatronymicChange,
  onPhoneChange,
  onCommentChange,
  onSubmit,
}: TenantFormStepProps): JSX.Element {
  const isNameValid = name.trim() !== '';
  const isCommentValid = comment.length <= MAX_COMMENT_LENGTH;
  const isPhoneFormatValid = isPhoneValid(phone);
  const canSubmit = isNameValid && isCommentValid && isPhoneFormatValid && !isLoading;

  return (
    <div className={styles.root}>
      <h2 className={styles.heading}>Добавление арендатора</h2>
      <div className={styles.fields}>
        <TextField
          label="Имя"
          placeholder=" "
          required
          value={name}
          onChange={(e) => onNameChange(e.currentTarget.value)}
          fullWidth
        />
        <TextField
          label="Фамилия"
          placeholder=" "
          value={surname}
          onChange={(e) => onSurnameChange(e.currentTarget.value)}
          fullWidth
        />
        <TextField
          label="Отчество"
          placeholder=" "
          value={patronymic}
          onChange={(e) => onPatronymicChange(e.currentTarget.value)}
          fullWidth
        />
        <TextField
          label="Комментарий"
          placeholder=" "
          multiline
          maxLength={MAX_COMMENT_LENGTH}
          value={comment}
          onChange={(e) => onCommentChange(e.currentTarget.value)}
          fullWidth
        />
        <TextField
          label="Телефон"
          placeholder=" "
          value={phone}
          onChange={(e) => onPhoneChange(e.currentTarget.value)}
          fullWidth
          error={!isPhoneFormatValid ? 'Введите корректный номер телефона' : undefined}
        />
      </div>
      {error && <p className={styles.error}>{error}</p>}
      <div className={styles.footer}>
        <Button
          type="button"
          variant="primary"
          size="large"
          fullWidth
          disabled={!canSubmit}
          loading={isLoading}
          onClick={onSubmit}
        >
          Добавить арендатора
        </Button>
      </div>
    </div>
  );
}
