'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import { ArrowDown, ArrowLeft, BoldHome, Edit, SortingBigSmall, SortingSmallBig } from '@/shared/assets/icons';
import { Button, ChipButton, IconButton } from '@/shared/ui/design';
import styles from '../../page.module.css';

const dlButtonVariants = ['primary', 'secondary', 'danger', 'clear', 'white'] as const;
const dlIconVariants = ['primary', 'secondary', 'danger'] as const;

export function ButtonSection(): JSX.Element {
    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>Button</h3>
            <div className={styles.grid}>
                {dlButtonVariants.map((variant) => (
                    <Button key={variant} variant={variant}>
                        {variant}
                    </Button>
                ))}
                <Button loading>loading</Button>
                <Button disabled>disabled</Button>
                <Button variant="primary" leadingIcon={<BoldHome />} trailingIcon={<Edit />}>
                    with icons
                </Button>
            </div>
            <div className={styles.grid}>
                {dlButtonVariants.map((variant) => (
                    <Button key={variant} variant={variant} size="small">
                        small
                    </Button>
                ))}
                <Button variant="secondary" size="small" selected>
                    selected
                </Button>
                <Button size="small" loading>
                    loading
                </Button>
                <Button size="small" disabled>
                    disabled
                </Button>
                {/* Радиус m (12px, токен Figma radius/m) — CTA пустых состояний
                 * секций объекта (#588, решение владельца 11.09). */}
                {dlButtonVariants.map((variant) => (
                    <Button key={`m-${variant}`} variant={variant} size="small" radius="m">
                        small · m
                    </Button>
                ))}
                <Button variant="primary" radius="m">
                    default · m
                </Button>
            </div>
        </div>
    );
}

export function IconButtonSection(): JSX.Element {
    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>IconButton</h3>
            <div className={styles.grid}>
                {dlIconVariants.map((variant) => (
                    <IconButton key={variant} variant={variant} icon={<ArrowLeft />} label={`Назад ${variant}`} />
                ))}
                <IconButton variant="primary" icon={<Edit />} label="Редактировать" disabled />
            </div>
        </div>
    );
}

export function ChipButtonSection(): JSX.Element {
    const [chip, setChip] = useState('name');

    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>ChipButton</h3>
            <div className={styles.grid}>
                <ChipButton
                    leadingIcon={<SortingSmallBig />}
                    trailingIcon={<ArrowDown />}
                    selected={chip === 'name'}
                    onClick={() => setChip('name')}
                >
                    По названию
                </ChipButton>
                <ChipButton
                    leadingIcon={<SortingBigSmall />}
                    trailingIcon={<ArrowDown />}
                    selected={chip === 'amount'}
                    onClick={() => setChip('amount')}
                >
                    По сумме
                </ChipButton>
                <ChipButton trailingIcon={<ArrowDown />} disabled>
                    disabled
                </ChipButton>
            </div>
        </div>
    );
}
