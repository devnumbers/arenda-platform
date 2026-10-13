'use client';

import type { JSX } from 'react';
import { DragScrollRail } from '@/shared/ui/drag-scroll';
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

/** Drag горизонтальных snap-лент мышью (#1349): pointer-прокси поверх
 * нативного скролла — лента тащится мышью/пером, отпускание доезжает
 * инерцией к snap-точке (inertia motion, power 0.8, timeConstant 500мс);
 * тач не заходит в код (нативный momentum/bounce и mandatory-снап целы);
 * reduced-motion доезд схлопывает в мгновенную установку. Смещение до 8px —
 * клик по плитке, дальше — прокрутка (click отпускания гасится). Резинка
 * за краями не имитируется — нативный скроллер батутит сам. Вживую: ленты
 * секций хаба «Платежи» (widgets/payments/ui/payments-global-screen.tsx).
 * Математика — чистый модуль drag-scroll-math (юнит-канон pull-math). */
export function DragScrollSection(): JSX.Element {
    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>DragScrollRail · drag лент мышью «как у Apple»</h3>
            <p className={styles.groupTitle}>
                Перетащите ленту мышью: во время жеста scroll-snap-type: none (штатный off-switch),
                scrollLeft следует за указателем; на отпускании — инерционный доезд к snap-точке,
                ближайшей к проекции остановки (веб-аналог targetContentOffset UIScrollView).
                Клик по плитке живёт: смещение до 8px — ещё клик. Колесо/тачпад и тач — нативные.
            </p>
            <div className={styles.column} style={{ maxWidth: 480 }}>
                {/* w-full: .column держит align-items: flex-start — без него
                 * лента сжимается до интринсик-ширины плиток и не переполняется. */}
                <DragScrollRail testId="ui-kit-drag-rail" className="w-full rounded-card">
                    {DEMO_TILES.map((tile) => (
                        <div
                            key={tile}
                            className="flex h-[132px] w-[168.5px] shrink-0 snap-start cursor-pointer items-end rounded-card bg-surface-muted p-4 text-sm font-medium text-content"
                        >
                            {tile}
                        </div>
                    ))}
                </DragScrollRail>
            </div>
        </div>
    );
}
