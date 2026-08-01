'use client';

import {
    type ChangeEvent,
    type FormEvent,
    type JSX,
    useCallback,
    useState,
} from 'react';
import {Button} from '@/shared/ui/button';
import {TextField} from '@/shared/ui/text-field';
import {formatPhoneInput, isPhoneValid, normalizePhone} from '@/shared/lib/phone';
import styles from './PropertyContactForm.module.css';

export interface PropertyContactFormData {
    name: string;
    phone: string;
}

export interface PropertyContactFormProps {
    readonly submitLabel: string;
    readonly isLoading: boolean;
    readonly onSubmit: (data: PropertyContactFormData) => void;
    readonly onCancel: () => void;
}

export function PropertyContactForm({
                                        submitLabel,
                                        isLoading,
                                        onSubmit,
                                        onCancel,
                                    }: PropertyContactFormProps): JSX.Element {
    const [name, setName] = useState('');
    const [phone, setPhone] = useState(formatPhoneInput(''));
    const [isSubmitAttempted, setIsSubmitAttempted] = useState(false);

    const isNameValid = name.trim() !== '';
    const isPhoneValidValue = isPhoneValid(phone);
    const canSubmit = isNameValid && isPhoneValidValue && !isLoading;

    const nameError = (isSubmitAttempted || name !== '') && !isNameValid
        ? 'Введите имя'
        : undefined;
    const phoneError = isSubmitAttempted && !isPhoneValidValue
        ? 'Введите корректный номер телефона'
        : undefined;

    const handlePhoneChange = useCallback((event: ChangeEvent<HTMLInputElement>) => {
        setPhone(formatPhoneInput(event.currentTarget.value));
    }, []);

    const handleSubmit = useCallback(
        (event: FormEvent<HTMLFormElement>) => {
            event.preventDefault();
            setIsSubmitAttempted(true);

            if (!canSubmit) {
                return;
            }

            onSubmit({
                name: name.trim(),
                phone: normalizePhone(phone),
            });
        },
        [canSubmit, name, onSubmit, phone],
    );

    const handleCancelClick = useCallback(() => {
        onCancel();
    }, [onCancel]);

    return (
        <form onSubmit={handleSubmit} className={styles.form}>
            <div className={styles.fields}>
                <TextField
                    label="Имя"
                    placeholder=" "
                    required
                    value={name}
                    onChange={(event) => setName(event.currentTarget.value)}
                    error={nameError}
                    fullWidth
                />
                <TextField
                    label="Телефон"
                    placeholder="+7 (000) 000-00-00"
                    required
                    value={phone}
                    onChange={handlePhoneChange}
                    error={phoneError}
                    fullWidth
                />
            </div>

            <div className={styles.actions}>
                <Button
                    type="submit"
                    variant="primary"
                    size="large"
                    fullWidth
                    loading={isLoading}
                    disabled={!canSubmit}
                >
                    {submitLabel}
                </Button>
                <Button
                    type="button"
                    variant="secondary"
                    size="large"
                    fullWidth
                    onClick={handleCancelClick}
                >
                    Отмена
                </Button>
            </div>
        </form>
    );
}
