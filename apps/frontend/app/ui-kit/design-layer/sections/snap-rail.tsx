'use client';

import type { JSX } from 'react';
import { SnapRail } from '@/shared/ui/snap-rail';
import styles from '../../page.module.css';

/** Демо-плитки ленты — габарит карточек платежей хаба (168.5×132). */
const DEMO_TILES = [
    'Избранное',
    'Просроченное',
    'На оплату',
    'Аренда',
    'Коммунальные',
    'Интернет',
    'Домофон',
    'Парковка',
] as const;

/** SnapRail — Embla dragFree: 'snap' (тикет #1349; канон механики —
 * DESIGN.md §8 «Drag горизонтальных snap-лент»): drag мышью/тачем с
 * инерцией и доездом к ближайшей snap-точке, резинка у краёв, трекпад —
 * wheel-gestures. Смещение до 10px — клик по плитке. Линия снапа здесь —
 * край вьюпорта (паддинга нет). */
export function SnapRailSection(): JSX.Element {
    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>SnapRail · Embla dragFree: &apos;snap&apos;</h3>
            <p className={styles.groupTitle}>
                Перетащите ленту мышью: инерционный доезд к ближайшей snap-точке;
                у краёв — резинка (сопротивление и мягкий возврат). Трекпад —
                двумя пальцами (wheel-gestures), вертикальное колесо и тач —
                штатно: страница скроллится, лента не перехватывает. Смещение
                до 10px — клик по плитке. Линия снапа здесь — край вьюпорта
                (паддинга нет).
            </p>
            <div className={styles.column} style={{ maxWidth: 480 }}>
                {/* w-full: .column держит align-items: flex-start — без него
                 * лента сжимается до интринсик-ширины плиток и не переполняется. */}
                <SnapRail testId="ui-kit-snap-rail" className="w-full rounded-card">
                    {DEMO_TILES.map((tile) => (
                        <div
                            key={tile}
                            className="flex h-[132px] w-[168.5px] shrink-0 cursor-pointer items-end rounded-card bg-surface-muted p-4 text-sm font-medium text-content"
                        >
                            {tile}
                        </div>
                    ))}
                </SnapRail>
            </div>
        </div>
    );
}
