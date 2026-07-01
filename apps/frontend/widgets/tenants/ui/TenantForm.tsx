'use client';

import {
    type ChangeEvent,
    type FormEvent,
    type JSX,
    type MouseEvent,
    useCallback,
    useEffect,
    useMemo,
    useState
} from 'react';
import {Button} from '@/shared/ui/button';
import {LinkButton} from '@/shared/ui/link-button';
import {TextField} from '@/shared/ui/text-field';
import {ArrowLeft} from '@/shared/assets/icons';
import {formatPhoneInput, normalizePhone} from '@/shared/lib/phone';
import styles from './TenantForm.module.css';

export interface TenantContactFormData {
    name: string;
    surname: string;
    patronymic: string;
    phone: string;
    email: string;
    comment: string;
}

export interface TenantFormProps {
    initialData?: Partial<TenantContactFormData>;
    submitLabel: string;
    isLoading: boolean;
    disabled?: boolean;
    error?: string;
    onSubmit: (data: TenantContactFormData) => void;
    onChange?: (data: TenantContactFormData) => void;
    onCancel?: () => void;
    backHref: string;
}

const MAX_COMMENT_LENGTH = 500;

const EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export function TenantForm({
                               initialData,
                               submitLabel,
                               isLoading,
                               disabled = false,
                               error,
                               onSubmit,
                               onChange,
                               onCancel,
                               backHref,
                           }: TenantFormProps): JSX.Element {
    const [name, setName] = useState(initialData?.name?.trim() ?? '');
    const [surname, setSurname] = useState(initialData?.surname?.trim() ?? '');
    const [patronymic, setPatronymic] = useState(initialData?.patronymic?.trim() ?? '');
    const [phone, setPhone] = useState(
        initialData?.phone ? formatPhoneInput(initialData.phone) : '',
    );
    const [email, setEmail] = useState(initialData?.email?.trim() ?? '');
    const [comment, setComment] = useState(initialData?.comment?.trim() ?? '');

    const [isPhoneTouched, setIsPhoneTouched] = useState(false);
    const [isEmailTouched, setIsEmailTouched] = useState(false);
    const [isSubmitAttempted, setIsSubmitAttempted] = useState(false);

    useEffect(() => {
        onChange?.({
            name,
            surname,
            patronymic,
            phone,
            email,
            comment,
        });
    }, [comment, email, name, onChange, patronymic, phone, surname]);

    const isNameValid = name.trim() !== '';
    const isPhoneValid = phone === '' || phone === '+7' || phone.length === 18;
    const isEmailValid = email === '' || EMAIL_REGEX.test(email);
    const isCommentValid = comment.length <= MAX_COMMENT_LENGTH;

    const hasChanges = useMemo(() => {
        if (!initialData) return true;
        return (
            name.trim() !== (initialData.name ?? '').trim() ||
            surname.trim() !== (initialData.surname ?? '').trim() ||
            patronymic.trim() !== (initialData.patronymic ?? '').trim() ||
            (phone.trim() === '' || phone.trim() === '+7' ? '' : normalizePhone(phone.trim())) !== (initialData.phone ?? '').trim() ||
            email.trim() !== (initialData.email ?? '').trim() ||
            comment.trim() !== (initialData.comment ?? '').trim()
        );
    }, [initialData, name, surname, patronymic, phone, email, comment]);

    const canSubmit = isNameValid && isPhoneValid && isEmailValid && isCommentValid && !isLoading && !disabled && hasChanges;

    const nameError = (isSubmitAttempted || name !== '') && !isNameValid ? 'Введите имя' : undefined;
    const phoneError = (isSubmitAttempted || isPhoneTouched) && !isPhoneValid
        ? 'Введите корректный номер телефона'
        : undefined;
    const emailError = (isSubmitAttempted || isEmailTouched) && !isEmailValid
        ? 'Введите корректный email'
        : undefined;

    const handlePhoneChange = useCallback((event: ChangeEvent<HTMLInputElement>) => {
        setIsPhoneTouched(true);
        setPhone(formatPhoneInput(event.currentTarget.value));
    }, []);

    const handleEmailChange = useCallback((event: ChangeEvent<HTMLInputElement>) => {
        setIsEmailTouched(true);
        setEmail(event.currentTarget.value);
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
                surname: surname.trim(),
                patronymic: patronymic.trim(),
                phone: phone.trim() === '' || phone.trim() === '+7' ? '' : normalizePhone(phone.trim()),
                email: email.trim(),
                comment: comment.trim(),
            });
        },
        [canSubmit, comment, email, name, onSubmit, patronymic, phone, surname],
    );

    const handleCancelClick = useCallback(
        (event: MouseEvent<HTMLAnchorElement>) => {
            if (onCancel) {
                event.preventDefault();
                onCancel();
            }
        },
        [onCancel],
    );

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
                    label="Фамилия"
                    placeholder=" "
                    value={surname}
                    onChange={(event) => setSurname(event.currentTarget.value)}
                    fullWidth
                />
                <TextField
                    label="Отчество"
                    placeholder=" "
                    value={patronymic}
                    onChange={(event) => setPatronymic(event.currentTarget.value)}
                    fullWidth
                />
                <TextField
                    label="Телефон"
                    placeholder="+7 (000) 000-00-00"
                    value={phone}
                    onChange={handlePhoneChange}
                    error={phoneError}
                    fullWidth
                />
                <TextField
                    label="Email"
                    placeholder="email@example.com"
                    value={email}
                    onChange={handleEmailChange}
                    error={emailError}
                    fullWidth
                />
                <TextField
                    label="Комментарий"
                    placeholder=" "
                    value={comment}
                    onChange={(event) => setComment(event.currentTarget.value)}
                    multiline
                    maxLength={MAX_COMMENT_LENGTH}
                    fullWidth
                />
            </div>

            {error && (
                <p className={styles.error} role="alert">
                    {error}
                </p>
            )}

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
            </div>
        </form>
    );
}
