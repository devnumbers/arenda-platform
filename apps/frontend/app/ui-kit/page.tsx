'use client';

import {type JSX, useState} from 'react';
import clsx from 'clsx';
import {Button} from '@/shared/ui/button';
import {IconButton} from '@/shared/ui/icon-button';
import {IconLink} from '@/shared/ui/icon-link';
import {LinkButton} from '@/shared/ui/link-button';
import {notify} from '@/shared/lib/notifications';
import {TextField} from '@/shared/ui/text-field';
import {ArrowRight, Home, Loading, Search, Settings, Support,} from '@/shared/assets/icons';
import {DesignLayerShowcase} from './design-layer';
import styles from './page.module.css';

const buttonVariants = ['primary', 'secondary', 'clear', 'icon-black'] as const;
const buttonSizes = ['large', 'medium', 'small', 'tiny'] as const;
const iconButtonVariants = ['primary', 'secondary', 'clear', 'icon-black', 'white-icon', 'primary-icon'] as const;

export default function UiKitPage(): JSX.Element {
    const [counterValue, setCounterValue] = useState('Hello');
    const [insideValue, setInsideValue] = useState('');

    return (
        <main className={styles.page}>
            <h1 className={styles.title}>UI Kit</h1>

            {/* Новые компоненты — дизайн-слой (ADR 0050): источник правды,
                всё новое строится на них. */}
            <DesignLayerShowcase/>

            {/* Старые компоненты (HeroUI) — под замену дизайн-слоем, будут
                удалены; показаны для сверки при миграции. */}
            <section className={styles.section}>
                <h2 className={clsx(styles.sectionTitle, styles.deprecatedTitle)}>
                    Старые компоненты (HeroUI) — под замену
                </h2>
                <p className={styles.statusNote}>
                    Устаревший слой: не использовать в новом коде (ADR 0050), будет удалён после миграции экранов.
                </p>
            </section>

            <section className={styles.section}>
                <h2 className={styles.sectionTitle}>Button</h2>
                {buttonVariants.map((variant) => (
                    <div key={variant} className={styles.group}>
                        <h3 className={styles.groupTitle}>variant=&quot;{variant}&quot;</h3>
                        <div className={styles.grid}>
                            {buttonSizes.map((size) => (
                                <Button key={size} variant={variant} size={size}>
                                    {size}
                                </Button>
                            ))}
                            {buttonSizes.map((size) => (
                                <Button key={`${size}-rounded`} variant={variant} size={size} rounded>
                                    {size}
                                </Button>
                            ))}
                            <Button variant={variant} loading>
                                loading
                            </Button>
                            <Button variant={variant} disabled>
                                disabled
                            </Button>
                            <Button
                                variant={variant}
                                size="medium"
                                leftIcon={<Home/>}
                                rightIcon={<ArrowRight/>}
                                subtitle="Subtitle"
                            >
                                With icons
                            </Button>
                        </div>
                    </div>
                ))}
            </section>

            <section className={styles.section}>
                <h2 className={styles.sectionTitle}>IconButton</h2>
                {iconButtonVariants.map((variant) => (
                    <div
                        key={variant}
                        className={clsx(styles.group, variant === 'white-icon' && styles.darkGroup)}
                    >
                        <h3 className={styles.groupTitle}>variant=&quot;{variant}&quot;</h3>
                        <div className={styles.grid}>
                            {buttonSizes.map((size) => (
                                <IconButton
                                    key={size}
                                    variant={variant}
                                    size={size}
                                    icon={<Search/>}
                                    aria-label={`Search ${size}`}
                                />
                            ))}
                            {buttonSizes.map((size) => (
                                <IconButton
                                    key={`${size}-rounded`}
                                    variant={variant}
                                    size={size}
                                    rounded
                                    icon={<Settings/>}
                                    aria-label={`Settings ${size}`}
                                />
                            ))}
                            <IconButton variant={variant} icon={<Loading/>} aria-label="Loading" loading/>
                        </div>
                    </div>
                ))}
            </section>

            <section className={styles.section}>
                <h2 className={styles.sectionTitle}>LinkButton</h2>
                <div className={styles.links}>
                    {buttonVariants.map((variant) => (
                        <LinkButton key={variant} href="#" variant={variant}>
                            {variant}
                        </LinkButton>
                    ))}
                    <LinkButton href="#" variant="primary" loading>
                        loading
                    </LinkButton>
                    <LinkButton href="#" variant="primary" leftIcon={<Home/>} rightIcon={<ArrowRight/>}>
                        with icons
                    </LinkButton>
                </div>
            </section>

            <section className={styles.section}>
                <h2 className={styles.sectionTitle}>IconLink</h2>
                <div className={styles.links}>
                    {iconButtonVariants.slice(0, 4).map((variant) => (
                        <IconLink
                            key={variant}
                            href="#"
                            variant={variant}
                            icon={<Support/>}
                            aria-label={`Support ${variant}`}
                        />
                    ))}
                    <IconLink href="#" variant="primary" icon={<Loading/>} aria-label="Loading" loading/>
                </div>
            </section>

            <section className={styles.section}>
                <h2 className={styles.sectionTitle}>TextField</h2>
                <div className={styles.textFields}>
                    <TextField label="Label outside" placeholder="Type something…"/>
                    <TextField label="Required" required placeholder="Required field"/>
                    <TextField placeholder="No label"/>
                    <TextField label="With icon" icon={<Search/>} placeholder="Search…"/>
                    <TextField
                        label="With helper"
                        helperText="This is a helper message"
                        placeholder="Helper text below"
                    />
                    <TextField
                        label="With error"
                        error="This field is invalid"
                        placeholder="Error state"
                    />
                    <TextField label="Disabled" disabled placeholder="Cannot type"/>
                    <TextField
                        label="Label inside"
                        labelPlacement="inside"
                        placeholder="Floating label"
                        value={insideValue}
                        onChange={(event) => setInsideValue(event.target.value)}
                    />
                    <TextField
                        label="Inside with icon"
                        labelPlacement="inside"
                        icon={<Search/>}
                        placeholder="Search…"
                    />
                    <TextField
                        label="Multiline"
                        multiline
                        helperText="Resize the textarea"
                        placeholder="Type a longer message…"
                    />
                    <TextField
                        label="With counter"
                        maxLength={20}
                        value={counterValue}
                        onChange={(event) => setCounterValue(event.target.value)}
                    />
                    <TextField
                        label="Counter exceeded"
                        maxLength={10}
                        value="This text is too long"
                        showCounter
                    />
                    <TextField
                        label="Full width"
                        fullWidth
                        placeholder="Spans the full width of its container"
                    />
                </div>
            </section>

            <section className={styles.section}>
                <h2 className={styles.sectionTitle}>Toast</h2>
                <div className={styles.grid}>
                    <Button onClick={() => notify.scenarios.demo.success({description: 'Изменения успешно сохранены'})}>
                        Success
                    </Button>
                    <Button onClick={() => notify.scenarios.demo.error({description: 'Не удалось сохранить изменения. Попробуйте ещё раз'})}>
                        Error
                    </Button>
                    <Button onClick={() => notify.scenarios.demo.info({description: 'Списание за тариф прошло успешно'})}>
                        Info
                    </Button>
                    <Button onClick={() => notify.scenarios.demo.warning({description: 'Срок действия подписки истекает через 7 дней'})}>
                        Warning
                    </Button>
                    <Button
                        onClick={() =>
                            notify.scenarios.paymentMethods.cardAdded({
                                description: 'Visa •• 4242',
                                action: {
                                    label: 'Открыть',
                                    onPress: () => console.log('action'),
                                },
                            })
                        }
                    >
                        With action
                    </Button>
                </div>
            </section>
        </main>
    );
}
