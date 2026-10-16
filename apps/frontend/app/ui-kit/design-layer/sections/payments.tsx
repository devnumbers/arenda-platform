'use client';

import type { JSX } from 'react';
import { Star, StarOff } from '@/shared/assets/icons';
import { IconButton } from '@/shared/ui/design';
import {
    CategoryIcon,
    categoryStyle,
    type PaymentCategoryEntry,
    paymentCategories,
} from '@/features/payment-categories';
import { PaymentCardButton, PaymentRowButton, formatOverdueDays } from '@/entities/payment';
import styles from '../../page.module.css';

/** Витринные категории из сгенерированного каталога #447. */
const showcaseCategorySlugs = [
    'rent',
    'utilities',
    'mortgage',
    'insurance',
    'damage-compensation',
    'late-fees',
] as const;

const showcaseCategories = showcaseCategorySlugs
    .map((slug) => paymentCategories.find((entry) => entry.slug === slug))
    .filter((entry) => entry !== undefined);

const showcaseBySlug = new Map<string, PaymentCategoryEntry>(
    showcaseCategories.map((entry) => [entry.slug, entry]),
);

/** Витрина демо-данными каталога: слаг гарантированно входит в набор выше. */
function showcaseEntry(slug: string): PaymentCategoryEntry {
    const entry = showcaseBySlug.get(slug);
    if (entry === undefined) {
        throw new Error(`showcase: в каталоге нет категории ${slug}`);
    }
    return entry;
}

export function CategoryIconSection(): JSX.Element {
    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>CategoryIcon — доменный композит категорий (#447)</h3>
            <p className={styles.groupTitle}>
                Круг 44×44 с цветом подложки каталога, кантом по поверхности и bold-иконкой;
                бейдж danger — просрочка на верхнем левом углу (Figma 651:5925).
            </p>
            <div className={styles.grid}>
                {showcaseCategories.map((entry) => (
                    <CategoryIcon key={entry.slug} icon={entry.icon} color={entry.color} />
                ))}
                <CategoryIcon
                    icon={showcaseEntry('late-fees').icon}
                    color={showcaseEntry('late-fees').color}
                    badge="danger"
                />
                <CategoryIcon {...categoryStyle('custom')} surface="muted" />
            </div>
        </div>
    );
}

export function PaymentButtonsSection(): JSX.Element {
    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>PaymentRowButton · PaymentCardButton — композиты платежей (#462)</h3>
            <p className={styles.groupTitle}>
                Строка операции (White/Gray) и карточка просроченного 168.5px; danger красит
                сумму и срок (#452). Слева — CategoryIcon с серым кантом на серой плитке.
            </p>
            <div className={styles.column} style={{ maxWidth: 480 }}>
                <PaymentRowButton
                    categoryIcon={
                        <CategoryIcon
                            icon={showcaseEntry('rent').icon}
                            color={showcaseEntry('rent').color}
                        />
                    }
                    title="Арендная плата"
                    subtitle="Моя квартира"
                    subtitleIcon={<Star aria-hidden />}
                    amountKopecks={5600000}
                    description="11 сентября"
                    onSelect={() => undefined}
                />
                <PaymentRowButton
                    variant="gray"
                    leading={<IconButton icon={<StarOff />} label="Избранное" />}
                    categoryIcon={
                        <CategoryIcon
                            icon={showcaseEntry('damage-compensation').icon}
                            color={showcaseEntry('damage-compensation').color}
                            badge="danger"
                            surface="muted"
                        />
                    }
                    title="Возмещение ущерба"
                    subtitle={formatOverdueDays(4)}
                    amountKopecks={320000}
                    description="17 августа"
                    danger
                    onSelect={() => undefined}
                />
                <div className="flex gap-2 overflow-x-auto py-1">
                    <PaymentCardButton
                        leading={
                            <CategoryIcon
                                icon={showcaseEntry('insurance').icon}
                                color={showcaseEntry('insurance').color}
                                surface="muted"
                            />
                        }
                        title="Страхование квартиры в центре города"
                        subtitle="Моя квартира в центре"
                        amountKopecks={3200000}
                        description="17 августа"
                        onSelect={() => undefined}
                    />
                    <PaymentCardButton
                        leading={
                            <CategoryIcon
                                icon={showcaseEntry('late-fees').icon}
                                color={showcaseEntry('late-fees').color}
                                badge="danger"
                                surface="muted"
                            />
                        }
                        title="Пени по аренде"
                        subtitle="Моя квартира"
                        amountKopecks={850000}
                        description={formatOverdueDays(3)}
                        danger
                        onSelect={() => undefined}
                    />
                </div>
            </div>
        </div>
    );
}
