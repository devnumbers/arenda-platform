'use client';

import { type JSX, useState } from 'react';
import { Support } from '@/shared/assets/icons';
import { Button, HeaderLogo, SupportModal } from '@/shared/ui/design';

/** Шапка шага входа — общий хром шагов почты #764 и кода #765 (с ADR 0067
 * #1529 поток двухшаговый; сведение копирайта — находка pre-merge карты
 * #761). Колонка с зазором 8: лого-вектор 112×28 — канон HeaderLogo;
 * заголовок H1 28/32; подпись 16/18 secondary. */
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
                className="w-full text-base"
                trailingIcon={<Support />}
                onClick={() => setSupportOpen(true)}
            >
                Написать в поддержку
            </Button>

            <SupportModal open={supportOpen} onOpenChange={setSupportOpen} />
        </>
    );
}
