'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import { Button, ChipButton } from '@/shared/ui/design';
import { LiveValue } from '@/shared/ui/live-value';
import styles from '../../page.module.css';

/** Демо-пара значений — роли участника, как у живых потребителей
 * канона C: пилюля доступа объекта и «Права участника». */
const DEMO_ROLES = ['Администратор', 'Наблюдатель'] as const;

/** Демо-пара разновысоких значений: пилюля та же, длинное значение в
 * ней переносится на вторую строку — движение высоты свапа видно глазом
 * (после 380мс коробка стоит на новой высоте, к settle прыжка нет). */
const DEMO_HEIGHTS = [
    'Одна строка',
    'Две строки — уже переносится',
] as const;

export function LiveValueSection(): JSX.Element {
    const [roleIndex, setRoleIndex] = useState(0);
    const [heightIndex, setHeightIndex] = useState(0);
    const [own, setOwn] = useState(false);
    const [refreshing, setRefreshing] = useState(false);
    const [mode, setMode] = useState<'crossfade' | 'flash'>('crossfade');

    // own и valueKey едут одним коммитом: зеркальный эффект LiveValue
    // (пропы в рефы) объявлен раньше эффекта смены значения, поэтому
    // машина диспетчит смену с уже обновлённым own.
    const swapRole = (own: boolean): void => {
        setOwn(own);
        setRoleIndex((index) => (index + 1) % DEMO_ROLES.length);
    };

    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>LiveValue · realtime-оживание значения</h3>
            <p className={styles.groupTitle}>
                Канон C (§8, тикет #880): чужая правка — dim 350мс → hold на перечитывание
                → blur-кроссфейд со вспышкой; «Своё изменение» (own) — без анимаций,
                вспышка и dim — сигнал «кто-то другой поменял». Режим flash
                (режим B) — для плотных списков: новое значение сразу, только
                вспышка (приглушения и перечитывания он не знает). «Начать перечитывание» включает фазу dim —
                смена значения в ней держит dim до конца hold, без смены dim снимается
                плавно. Размер коробки на свапе едет от старого значения к новому
                (решение 28.09 — «как у Apple»). Демо самодостаточное: анимируется
                любая смена valueKey, бэкенд не нужен. Разновысокие значения: после
                движения (380мс) коробка стоит на новой высоте, к settle (1250мс)
                прыжка нет. Вживую: пилюля доступа объекта
                (widgets/property-detail/ui/PropertyAccessPill.tsx), «Права участника»
                (widgets/participants/ui/participant-rights-screen.tsx).
            </p>
            <div className={styles.column} style={{ maxWidth: 480 }}>
                <div className="flex items-center gap-3 text-sm text-content-secondary">
                    Роль участника:
                    <LiveValue valueKey={roleIndex} own={own} refreshing={refreshing} mode={mode}>
                        <span className="flex items-center rounded-pill bg-surface-muted px-3 py-1.5 text-sm font-medium text-content">
                            {DEMO_ROLES[roleIndex]}
                        </span>
                    </LiveValue>
                </div>
                <div className={styles.grid}>
                    <Button onClick={() => swapRole(false)}>Чужая правка</Button>
                    <Button onClick={() => swapRole(true)}>Своё изменение</Button>
                    <Button variant="secondary" onClick={() => setRefreshing((value) => !value)}>
                        {refreshing ? 'Остановить перечитывание' : 'Начать перечитывание'}
                    </Button>
                </div>
                <div className={styles.links}>
                    <span className="text-sm text-content-secondary">Режим подачи:</span>
                    <ChipButton selected={mode === 'crossfade'} onClick={() => setMode('crossfade')}>
                        crossfade
                    </ChipButton>
                    <ChipButton selected={mode === 'flash'} onClick={() => setMode('flash')}>
                        flash
                    </ChipButton>
                </div>
                <div className="flex items-center gap-3 text-sm text-content-secondary">
                    Разная высота:
                    <LiveValue valueKey={heightIndex} mode={mode}>
                        <span className="flex max-w-[200px] items-center rounded-pill bg-surface-muted px-3 py-1.5 text-sm font-medium text-content">
                            {DEMO_HEIGHTS[heightIndex]}
                        </span>
                    </LiveValue>
                    <Button onClick={() => setHeightIndex((index) => (index + 1) % DEMO_HEIGHTS.length)}>
                        Сменить значение
                    </Button>
                </div>
            </div>
        </div>
    );
}
