'use client';

import { type JSX, useState } from 'react';
import { formatCountdown } from '@/shared/lib/countdown';
import { Support } from '@/shared/assets/icons';
import { Button, HeaderLogo, SupportModal } from '@/shared/ui/design';

/** Шапка шага входа — общий хром трёх шагов (телефон #763, почта #764,
 * код #765; сведение копирайта — находка pre-merge карты #761). Колонка
 * с зазором 8: лого-вектор 112×28 — канон HeaderLogo; заголовок H1 28/32;
 * подпись 16/18 secondary. */
export function StepHeader({
    title,
    subtitle,
}: {
    readonly title: string;
    readonly subtitle: string;
}): JSX.Element {
    return (
        <>
            <HeaderLogo className="h-7 w-28" />

            <div className="flex flex-col gap-2">
                <h1 className="m-0 text-[28px] font-semibold leading-8 text-content">{title}</h1>
                <p className="m-0 text-base leading-[18px] text-content-secondary">{subtitle}</p>
            </div>
        </>
    );
}

/** Кулдаун-подпись resend-канона #733 «Запросить код можно через ММ:СС» —
 * моно 14/16 tertiary по центру; вне кулдауна не рисуется. Не часть
 * ResendCodeTile: там другой текст («новый код») и композиция
 * кнопка+подпись. */
export function CooldownNote({ secondsLeft }: { readonly secondsLeft: number }): JSX.Element | null {
    if (secondsLeft <= 0) {
        return null;
    }

    return (
        <p className="m-0 text-center font-mono text-sm font-medium leading-4 text-content-tertiary">
            Запросить код можно через {formatCountdown(secondsLeft)}
        </p>
    );
}

/** «Написать в поддержку» шагов входа — White-пилюля с Icon/R/Support,
 * открывает модалку «Связаться с нами» (#766). Владение модалкой у
 * триггера — канон поверхностей (#766: пилюля десктопа, шит «Еще», кнопка
 * «Тарифа» держат свои SupportModal так же); у шага кода кнопки в макете
 * нет — там компонент не используется. */
export function SupportRequestButton(): JSX.Element {
    const [supportOpen, setSupportOpen] = useState(false);

    return (
        <>
            <Button
                variant="white"
                className="w-full"
                trailingIcon={<Support />}
                onClick={() => setSupportOpen(true)}
            >
                Написать в поддержку
            </Button>

            <SupportModal open={supportOpen} onOpenChange={setSupportOpen} />
        </>
    );
}
