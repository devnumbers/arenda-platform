'use client';

import type { JSX } from 'react';
import { SnapRail } from '@/shared/ui/snap-rail';
import styles from '../../page.module.css';

/** Демо-плитки ленты — габарит карточек платежей хаба (168.5×137). */
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

/** SnapRail — нативный тач + drag-прокси мыши/пера с физикой Apple
 * (тикет #1349; канон механики — DESIGN.md §8 «Drag горизонтальных
 * snap-лент»): мышь тащит ленту с инерционным доездом к ближайшей
 * snap-точке и резинкой у краёв; тач и колесо ведёт браузер. Смещение
 * до 8px — клик по плитке. Линия снапа здесь — край вьюпорта (паддинга
 * нет). */
export function SnapRailSection(): JSX.Element {
    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>SnapRail · нативный тач + drag-прокси</h3>
            <p className={styles.groupTitle}>
                Перетащите ленту мышью: инерционный доезд к ближайшей snap-точке
                (проекция остановки d=0.998, спринг damping 1.0 / response 0.4);
                у краёв — резинка c=0.55 (сопротивление и мягкий возврат).
                Тач и трекпад — штатный нативный скролл браузера: страница
                скроллится по вертикали, лента — по горизонтали. Смещение до
                8px — клик по плитке. Линия снапа здесь — край вьюпорта
                (паддинга нет).
            </p>
            <div className={styles.column} style={{ maxWidth: 480 }}>
                {/* w-full: .column держит align-items: flex-start — без него
                 * лента сжимается до интринсик-ширины плиток и не переполняется. */}
                <SnapRail testId="ui-kit-snap-rail" className="w-full rounded-card">
                    {DEMO_TILES.map((tile) => (
                        <div
                            key={tile}
                            className="flex h-[137px] w-[168.5px] shrink-0 cursor-pointer items-end rounded-card bg-surface-muted p-4 text-sm font-medium text-content"
                        >
                            {tile}
                        </div>
                    ))}
                </SnapRail>
            </div>
        </div>
    );
}
