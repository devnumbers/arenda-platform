'use client';

import type { JSX } from 'react';
import { SnapRail, SNAP_TILE_CLASSES } from '@/shared/ui/snap-rail';
import styles from '../../page.module.css';

/** Демо-плитки ленты — габарит карточек платежей хаба (fluid: 2 на
 * мобайл-ярусе, 3 на планшете/ПК, 137 — высота худшего случая контента). */
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
                (проекция остановки d=0.998, критическое демпфирование без
                перелёта, старт со скоростью жеста — адаптивная ω); у краёв —
                резинка c=0.55 (сопротивление и пружинистый возврат).
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
                            className={`flex h-[137px] shrink-0 cursor-pointer items-end rounded-card bg-surface-muted p-4 text-sm font-medium text-content ${SNAP_TILE_CLASSES}`}
                        >
                            {tile}
                        </div>
                    ))}
                </SnapRail>
            </div>
        </div>
    );
}
