'use client';

import {useParams, useRouter} from 'next/navigation';
import type {JSX} from 'react';
import Image from 'next/image';
import {ROUTES} from '@/shared/config/routes';
import {canMutateProperty, propertyTypeLabels, useProperty} from '@/features/properties';
import {formatAttributesForCardGrouped} from '@/features/property-attributes';
import {
    IconButton,
    Skeleton,
    SubScreenShell,
} from '@/shared/ui/design';
import {Edit} from '@/shared/assets/icons';
import {Button} from '@/shared/ui/design';
import {propertySectionImages} from '../lib/property-sections';
import {PropertySectionCard} from './PropertySectionCard';

type DataField = {
    readonly label: string;
    readonly value: string;
};

/** Экран «Об объекте» (карта #583, тикет #588; шаблон владельца
 * 1550:95856 — заполненный, 1550:97124 — пустые характеристики): данные
 * объекта (название, адрес, тип) и характеристики read-only; карандаш в
 * шапке ведёт на правку, у архивного (read-only, ADR 0028) его нет. Вход —
 * пункт кебаба «Об объекте» и секция «Квартира» на детали. */
export function PropertyAboutScreen(): JSX.Element {
    const params = useParams<{ id: string }>();
    const id = params.id;
    const router = useRouter();

    const propertyQuery = useProperty(id);
    const property = propertyQuery.data;
    const canMutate = canMutateProperty(propertyQuery.isSuccess ? property : undefined);

    if (propertyQuery.isPending) {
        return (
            <SubScreenShell title="Об объекте" fallbackHref={ROUTES.property(id)}>
                <AboutLoading/>
            </SubScreenShell>
        );
    }

    if (propertyQuery.isError || !property) {
        return (
            <SubScreenShell title="Об объекте" fallbackHref={ROUTES.property(id)}>
                <div className="flex justify-center pt-16">
                    <Button
                        variant="secondary"
                        size="small"
                        onClick={() => void propertyQuery.refetch()}
                        disabled={propertyQuery.isFetching}
                    >
                        Не удалось загрузить объект. Повторить
                    </Button>
                </div>
            </SubScreenShell>
        );
    }

    const dataFields: ReadonlyArray<DataField> = [
        {label: 'Название', value: property.name},
        {label: 'Адрес', value: property.address},
        {label: 'Тип', value: propertyTypeLabels[property.type]},
    ];
    const groups = formatAttributesForCardGrouped(property.type, property.attributes);
    const hasAttributes = groups.length > 0;

    return (
        <SubScreenShell
            title="Об объекте"
            fallbackHref={ROUTES.property(id)}
            trailing={
                canMutate ? (
                    <IconButton
                        icon={<Edit className="h-6 w-6"/>}
                        label="Редактировать объект"
                        onClick={() => router.push(ROUTES.propertyEdit(id))}
                    />
                ) : undefined
            }
        >
            <PropertySectionCard title="Данные" className="mt-6">
                <dl className="flex flex-col gap-4 px-6 pt-1">
                    {dataFields.map((field) => (
                        <div key={field.label}>
                            <dt className="text-sm text-content-secondary">{field.label}</dt>
                            <dd className="mt-1 text-base leading-4 text-content">{field.value}</dd>
                        </div>
                    ))}
                </dl>
            </PropertySectionCard>

            <PropertySectionCard title="Характеристики">
                {hasAttributes ? (
                    <div className="flex flex-col gap-4 px-6 pt-1">
                        {groups.map((group, index) => (
                            <div key={group.group ?? `group-${index}`}>
                                {group.label !== null && (
                                    <h3 className="text-sm font-semibold text-content">{group.label}</h3>
                                )}
                                <dl className={`flex flex-col gap-2 ${group.label !== null ? 'mt-3' : ''}`}>
                                    {group.items.map((item) => (
                                        <div
                                            key={item.label}
                                            className="grid grid-cols-2 gap-2 text-sm"
                                        >
                                            <dt className="text-content-secondary">{item.label}</dt>
                                            <dd className="text-content">{item.value}</dd>
                                        </div>
                                    ))}
                                </dl>
                            </div>
                        ))}
                        <div>
                            <dl>
                                <dt className="text-sm text-content-secondary">Описание</dt>
                                <dd className="mt-1 text-base leading-4 text-content">
                                    {property.description ?? 'Не указано'}
                                </dd>
                            </dl>
                        </div>
                    </div>
                ) : (
                    <div className="flex flex-col items-center px-6 pt-10 pb-2 text-center">
                        <Image
                            src={propertySectionImages.about}
                            alt=""
                            width={64}
                            height={64}
                            className="h-16 w-16"
                        />
                        <p className="mt-4 max-w-[281px] text-sm leading-5 text-content-secondary">
                            Не добавлены. Укажите площадь, этаж и другие параметры объекта
                        </p>
                        {canMutate && (
                            <Button
                                variant="primary"
                                size="small"
                                className="mt-6"
                                onClick={() => router.push(ROUTES.propertyEdit(id))}
                            >
                                Добавить
                            </Button>
                        )}
                    </div>
                )}
            </PropertySectionCard>
        </SubScreenShell>
    );
}

/** Скелетон «Об объекте» — геометрия двух карточек (§7, гейт Loading
 * stability): шапка-строка + ряды данных/атрибутов. */
function AboutLoading(): JSX.Element {
    return (
        <>
            <div className="mt-6 rounded-card bg-surface-muted px-6 pb-6 pt-3">
                <Skeleton className="h-5 w-24 bg-surface-muted-hover"/>
                <div className="mt-4 flex flex-col gap-4">
                    <Skeleton className="h-9 bg-surface-muted-hover"/>
                    <Skeleton className="h-9 bg-surface-muted-hover"/>
                    <Skeleton className="h-9 bg-surface-muted-hover"/>
                </div>
            </div>
            <div className="mt-4 rounded-card bg-surface-muted px-6 pb-6 pt-3">
                <Skeleton className="h-5 w-36 bg-surface-muted-hover"/>
                <div className="mt-4 flex flex-col gap-2">
                    <Skeleton className="h-4 bg-surface-muted-hover"/>
                    <Skeleton className="h-4 bg-surface-muted-hover"/>
                    <Skeleton className="h-4 bg-surface-muted-hover"/>
                    <Skeleton className="h-4 w-2/3 bg-surface-muted-hover"/>
                </div>
            </div>
        </>
    );
}
