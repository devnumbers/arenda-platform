'use client';

import type { JSX, ReactNode } from 'react';
import { ArrowLeft, Cancel } from '@/shared/assets/icons';
import { IconButton } from '@/shared/ui/design';

/**
 * Шейл экранов входа по макетам «Рентли. Новые экраны сервиса» (Figma
 * 2343:66417 — десктоп, 2343:67107 — планшет, 2343:67172 — мобайл;
 * карта #761, тикет #763). Фундамент редизайна: раскладки трёх брейкпоинтов
 * и бренд-ассеты (3D-стеклянная иконка и мягкий градиент — экспорт из
 * Figma, imageRef 6ca2192c…/be2ffd8a…), переиспользуются шагами почты
 * (#764) и кода (#765).
 *
 * Мобайл 320–560 — белая страница, контент сверху (pt-72 под верхний бар).
 * Планшет 561–1023 — градиент на всю страницу, размытый 3D-домик 700×700
 * по центру, белая карточка radius 32 padding 32 (max-w-720).
 * ПК ≥1024 — белый лист, две панели 50/50 с зазором и отступом 24:
 * слева градиент radius 32 с домиком 360 по центру, справа — контент
 * колонкой max-w-400 по центру. Крестик × — выход на лендинг (`onClose`),
 * во всех макетах в баре 72px справа; в standalone PWA скрывается
 * (`hideClose`) — закрывать некуда (решение владельца 18.09, текущее
 * поведение). Опциональная стрелка ← (`onBack`) — в том же баре слева
 * (макеты шага кода #765: 2349:67383/67411; на планшете 2349:67396 не
 * нарисована — рисуется для единообразия, сводить на приёмке): шаг кода
 * ведёт на предыдущий шаг — почта, если была, иначе телефон.
 */

export type LoginShellProps = {
    readonly onClose: () => void;
    /** Стрелка ← в баре слева: назад на предыдущий шаг (шаг кода #765). */
    readonly onBack?: () => void;
    readonly hideClose?: boolean;
    readonly children: ReactNode;
};

export function LoginShell({ onClose, onBack, hideClose = false, children }: LoginShellProps): JSX.Element {
    return (
        <main className="relative min-h-dvh bg-surface">
            {/* Планшетный фон: градиент + размытый домик 700×700 по центру. */}
            <div aria-hidden className="absolute inset-0 hidden tablet:block desktop:hidden">
                <div className="absolute inset-0 bg-[url('/images/login-panel-bg.png')] bg-cover bg-center" />
                <div className="absolute left-1/2 top-1/2 h-[700px] w-[700px] -translate-x-1/2 -translate-y-1/2 bg-[url('/images/login-house-3d.png')] bg-contain bg-center bg-no-repeat" />
            </div>

            <div className="relative flex min-h-dvh items-start justify-center px-6 pb-6 pt-[72px] tablet:items-center tablet:py-6 desktop:gap-6 desktop:p-6">
                {/* ПК: левая панель — градиент radius 32 с домиком 360 по центру. */}
                <div
                    aria-hidden
                    className="relative hidden flex-1 self-stretch overflow-hidden rounded-[32px] bg-[url('/images/login-panel-bg.png')] bg-cover bg-center desktop:block"
                >
                    <div className="absolute left-1/2 top-1/2 h-[360px] w-[360px] -translate-x-1/2 -translate-y-1/2 bg-[url('/images/login-house-3d.png')] bg-contain bg-center bg-no-repeat" />
                </div>

                {/* Правая половина (ПК) / вся страница (мобайл, планшет). */}
                <div className="flex w-full flex-1 items-center justify-center">
                    {/* Планшет: белая карточка radius 32 padding 32. */}
                    <div className="w-full max-w-[400px] tablet:max-w-[720px] tablet:rounded-[32px] tablet:bg-surface tablet:p-8">
                        <div className="mx-auto w-full desktop:max-w-[400px]">{children}</div>
                    </div>
                </div>
            </div>

            {(onBack !== undefined || !hideClose) && (
                <div className="absolute inset-x-0 top-0 z-10 flex h-[72px] items-center justify-between px-3">
                    {/* Левый слот держит × справа и когда стрелки нет. */}
                    {onBack !== undefined ? (
                        <IconButton icon={<ArrowLeft />} label="Назад" onClick={onBack} />
                    ) : (
                        <span aria-hidden />
                    )}
                    {!hideClose && <IconButton icon={<Cancel />} label="Закрыть" onClick={onClose} />}
                </div>
            )}
        </main>
    );
}
