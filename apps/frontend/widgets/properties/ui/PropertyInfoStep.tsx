'use client';

import {type JSX} from 'react';
import {Button} from '@/shared/ui/button';
import {TextField} from '@/shared/ui/text-field';
import styles from './PropertyInfoStep.module.css';

export type PropertyInfoStepProps = {
    name?: string;
    description?: string;
    onNameChange: (name: string) => void;
    onDescriptionChange: (description: string) => void;
    onSubmit: () => void;
    isLoading?: boolean;
};

export function PropertyInfoStep({
                                     name,
                                     description,
                                     onNameChange,
                                     onDescriptionChange,
                                     onSubmit,
                                     isLoading,
                                 }: PropertyInfoStepProps): JSX.Element {
    const isNameEmpty = (name?.trim() ?? '').length === 0;
    const canSubmit = !isNameEmpty && !isLoading;

    return (
        <div className={styles.root}>
            <h2 className={styles.heading}>Информация об объекте</h2>

            <TextField
                label="Название"
                required
                fullWidth
                value={name ?? ''}
                onChange={(event) => onNameChange(event.currentTarget.value)}
            />

            <TextField
                label="Описание"
                multiline
                fullWidth
                maxLength={500}
                value={description ?? ''}
                onChange={(event) => onDescriptionChange(event.currentTarget.value)}
            />

            <Button
                type="button"
                variant="primary"
                size="large"
                fullWidth
                loading={isLoading}
                disabled={!canSubmit}
                onClick={onSubmit}
                className={styles.submit}
            >
                Создать объект
            </Button>
        </div>
    );
}
