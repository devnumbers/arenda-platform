'use client';

import type {JSX} from 'react';
import {Button} from '@/shared/ui/design';
import {notify} from '@/shared/lib/notifications';
import {DesignLayerShowcase} from './design-layer';
import styles from './page.module.css';

export default function UiKitPage(): JSX.Element {
    return (
        <main className={styles.page}>
            <h1 className={styles.title}>UI Kit</h1>

            {/* Дизайн-слой (ADR 0050) — единственный источник правды:
                легаси-киты shared/ui снесены (тикет #901), витрина
                показывает только канон. */}
            <DesignLayerShowcase/>

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
                                description: '•••• 4242',
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
