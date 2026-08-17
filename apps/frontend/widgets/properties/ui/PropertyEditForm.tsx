'use client';

import {type FormEvent, type JSX, useCallback, useEffect, useMemo, useRef, useState,} from 'react';
import {useRouter} from 'next/navigation';
import {notify} from '@/shared/lib/notifications';
import {ROUTES} from '@/shared/config/routes';
import {goBack} from '@/shared/lib/navigation';
import {useProperty, useUpdateProperty} from '@/features/properties';
import {TextField} from '@/shared/ui/text-field';
import {Button} from '@/shared/ui/button';
import {PageHeader} from '@/shared/ui/page-header';
import type {PropertyAttributes, PropertyType} from '@/entities/property';
import {coerceAttributes} from '@/entities/property';
import {
    PropertyAttributesFields,
    validateAttributes,
    filterByType,
    fieldsForType,
    type AttrErrors,
    type AttrKey,
} from '@/features/property-attributes';
import {propertyTypeLabels} from '@/features/properties';
import {ApiError} from '@/shared/api/errors';
import {PropertyTypeSelect} from './PropertyTypeSelect';
import {AddressField} from './AddressField';
import {PropertyEditFormLoading} from './PropertyEditFormLoading';
import styles from './PropertyEditForm.module.css';

const MAX_NAME_LENGTH = 50;
const MAX_DESCRIPTION_LENGTH = 500;

export type PropertyEditFormProps = {
    readonly propertyId: string;
};

type PropertyEditFormErrorProps = {
    readonly onRetry: () => void;
    readonly isLoading?: boolean;
};

function PropertyEditFormError({
                                   onRetry,
                                   isLoading = false,
                               }: PropertyEditFormErrorProps): JSX.Element {
    return (
        <div className={styles.errorCard} role="alert" aria-live="polite">
            <h2 className={styles.errorTitle}>Не удалось загрузить объект</h2>
            <p className={styles.errorMessage}>
                Проверьте соединение и попробуйте снова
            </p>
            <Button
                variant="primary"
                size="medium"
                loading={isLoading}
                onClick={onRetry}
                type="button"
            >
                Повторить
            </Button>
        </div>
    );
}

function attributesEqual(a: PropertyAttributes, b: PropertyAttributes): boolean {
    const aKeys = Object.keys(a);
    const bKeys = Object.keys(b);
    if (aKeys.length !== bKeys.length) return false;
    return aKeys.every((key) => a[key] === b[key]);
}

export function PropertyEditForm({
                                     propertyId,
                                 }: PropertyEditFormProps): JSX.Element {
    const router = useRouter();

    const propertyQuery = useProperty(propertyId);
    const updateProperty = useUpdateProperty();

    const [type, setType] = useState<PropertyType | undefined>(undefined);
    const [address, setAddress] = useState('');
    const [name, setName] = useState('');
    const [description, setDescription] = useState('');
    const [attributes, setAttributes] = useState<PropertyAttributes>({});
    const [attrErrors, setAttrErrors] = useState<AttrErrors>({});
    const [typeChangeNotice, setTypeChangeNotice] = useState<string | null>(null);
    const [submitAttempted, setSubmitAttempted] = useState(false);
    const hasInitialized = useRef(false);

    useEffect(() => {
        const property = propertyQuery.data;
        if (!property || hasInitialized.current) return;

        // Initialize form state from loaded property data once to avoid wiping
        // user edits on background refetch.
        setType(property.type);
        setAddress(property.address);
        setName(property.name);
        setDescription(property.description ?? '');
        setAttributes(coerceAttributes(property.attributes));
        hasInitialized.current = true;
    }, [propertyQuery.data]);

    const isSubmitting = updateProperty.isPending;

    const isNameValid = name.trim().length > 0;
    const isAddressValid = address.trim().length > 0;
    const isTypeValid = type !== undefined;

    const handleTypeChange = (nextType: PropertyType) => {
        const prevType = type;
        setType(nextType);
        // Show notice only when switching type away from one that has filled
        // attributes with keys that don't belong to the new type. Data is NOT
        // deleted — foreign keys stay in state (lossless); filterByType() prunes
        // them only for display and submission.
        if (prevType && prevType !== nextType) {
            const prevFields = new Set<string>(fieldsForType(prevType).map((f) => f.key));
            const nextFields = new Set<string>(fieldsForType(nextType).map((f) => f.key));
            const hasForeign = Object.keys(attributes).some(
                (k) => prevFields.has(k) && !nextFields.has(k),
            );
            if (hasForeign) {
                setTypeChangeNotice(
                    `Характеристики, заполненные для типа «${propertyTypeLabels[prevType]}», сохранятся, но будут скрыты`,
                );
            }
        }
        // Fields of a different type — reset visible errors until next blur.
        setAttrErrors({});
    };

    const liveAttrErrors = useMemo(
        () => (type ? validateAttributes(type, attributes) : {}),
        [type, attributes],
    );
    const attrIsValid = Object.keys(liveAttrErrors).length === 0;

    const handleFieldBlur = useCallback(() => {
        if (!type) return;
        setAttrErrors(validateAttributes(type, attributes));
    }, [type, attributes]);

    const handleAttributesChange = useCallback(
        (next: PropertyAttributes) => {
            setAttributes((prev) => {
                const currentTypeKeys = filterByType(type!, prev);
                return {...currentTypeKeys, ...next};
            });
        },
        [type],
    );

    const hasChanges = useMemo(() => {
        const property = propertyQuery.data;
        if (!property) return false;

        const attributesChanged = !attributesEqual(attributes, property.attributes);

        return (
            type !== property.type ||
            name.trim() !== property.name ||
            address.trim() !== property.address ||
            (description.trim() || undefined) !== (property.description ?? undefined) ||
            attributesChanged
        );
    }, [type, name, address, description, attributes, propertyQuery.data]);

    const canSubmit =
        isNameValid && isAddressValid && isTypeValid && attrIsValid && !isSubmitting && hasChanges;

    const typeError = submitAttempted && !isTypeValid ? 'Выберите тип объекта' : undefined;
    const addressError = submitAttempted && !isAddressValid ? 'Укажите адрес' : undefined;
    const nameError = submitAttempted && !isNameValid ? 'Укажите название' : undefined;

    const visibleAttrErrors: AttrErrors = submitAttempted
        ? {...attrErrors, ...liveAttrErrors}
        : attrErrors;

    const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault();

        setSubmitAttempted(true);

        if (!canSubmit || !type) return;

        try {
            await updateProperty.mutateAsync({
                id: propertyId,
                data: {
                    name: name.trim(),
                    type,
                    address: address.trim(),
                    description: description.trim() || undefined,
                    attributes: filterByType(type, attributes),
                },
            });

            notify.scenarios.property.updated();
            goBack(router, ROUTES.property(propertyId));
        } catch (error: unknown) {
            if (error instanceof ApiError && error.fieldErrors && error.fieldErrors.length > 0) {
                const mapped: AttrErrors = {};
                for (const fe of error.fieldErrors) {
                    mapped[fe.field as AttrKey] = fe.detail;
                }
                setAttrErrors(mapped);
                setSubmitAttempted(true);
            }
            notify.scenarios.property.saveError();
        }
    };

    if (propertyQuery.isPending) {
        return <PropertyEditFormLoading />;
    }

    if (propertyQuery.isError || !propertyQuery.data) {
        return (
            <PropertyEditFormError
                onRetry={propertyQuery.refetch}
                isLoading={propertyQuery.isFetching}
            />
        );
    }

    return (
        <form className={styles.root} onSubmit={handleSubmit}>
            <PageHeader title="Информация об объекте" backHref={ROUTES.property(propertyId)}/>

            <section className={styles.section}>
                <h2 className={styles.sectionTitle}>Данные</h2>
                <div className={styles.fields}>
                    <PropertyTypeSelect value={type} onChange={handleTypeChange} error={typeError}/>
                    <AddressField value={address} onChange={setAddress} error={addressError}/>
                    <TextField
                        label="Название"
                        required
                        fullWidth
                        maxLength={MAX_NAME_LENGTH}
                        value={name}
                        error={nameError}
                        onChange={(event) => setName(event.currentTarget.value)}
                    />
                    <TextField
                        label="Описание"
                        multiline
                        fullWidth
                        maxLength={MAX_DESCRIPTION_LENGTH}
                        value={description}
                        onChange={(event) => setDescription(event.currentTarget.value)}
                    />
                </div>
            </section>

            {type && (
                <section className={styles.section}>
                    <div className={styles.sectionHeader}>
                        <h2 className={styles.sectionTitle}>Характеристики</h2>
                        {Object.keys(attributes).length > 0 && (
                            <Button
                                type="button"
                                variant="clear"
                                size="small"
                                onClick={() => {
                                    setAttributes({});
                                    setAttrErrors({});
                                }}
                            >
                                Очистить все
                            </Button>
                        )}
                    </div>
                    {typeChangeNotice && (
                        <div className={styles.notice} role="status" aria-live="polite">
                            <span className={styles.noticeText}>{typeChangeNotice}</span>
                            <button
                                type="button"
                                className={styles.noticeClose}
                                aria-label="Скрыть уведомление"
                                onClick={() => setTypeChangeNotice(null)}
                            >
                                ×
                            </button>
                        </div>
                    )}
                    <div className={styles.fields}>
                        <PropertyAttributesFields
                            type={type}
                            value={filterByType(type, attributes)}
                            onChange={handleAttributesChange}
                            errors={visibleAttrErrors}
                            onFieldBlur={handleFieldBlur}
                        />
                    </div>
                </section>
            )}

            <Button
                type="submit"
                variant="primary"
                size="large"
                fullWidth
                loading={isSubmitting}
                disabled={!canSubmit}
            >
                Сохранить изменения
            </Button>
        </form>
    );
}
