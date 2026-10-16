'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import { Add, Filter } from '@/shared/assets/icons';
import { IconButton, SearchPill } from '@/shared/ui/design';
import styles from '../../page.module.css';

export function SearchPillSection(): JSX.Element {
    const [lastAction, setLastAction] = useState('ничего не нажато');

    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>SearchPill · пилюля поиска хаба</h3>
            <p className={styles.groupTitle}>
                Канон Search Button: серый rounded-pill на всю ширину колонки, лупа слева,
                подпись, опциональный хвост. Кликается вся пилюля целиком и активируется
                с клавиатуры (строка — div с role=button, потому что хвост несёт собственные
                кнопки, а вложенные кнопки в HTML невалидны). Кнопки хвоста глушат всплытие
                клика сами (stopPropagation) — клик и Enter/Space на хвосте не открывают
                поисковую страницу (#831); декоративные иконки хвоста (слайдеры фильтра)
                aria-hidden и клика не перехватывают. Вживую: «Платежи» и «Операции»
                (widgets/payments/ui/payments-global-screen.tsx, operations-global-screen.tsx),
                книга «Контакты» (widgets/contacts/ui/contact-book-screen.tsx).
            </p>
            <div className={styles.column} style={{ maxWidth: 560 }}>
                <SearchPill
                    label="Найти платёж"
                    onOpenSearch={() => setLastAction('открыт поиск платежей')}
                    className="pr-4"
                    trailing={
                        <Filter className="h-6 w-6 shrink-0 text-content" aria-hidden />
                    }
                />
                <SearchPill
                    label="Найти контакт"
                    onOpenSearch={() => setLastAction('открыт поиск контактов')}
                    className="pr-2"
                    trailing={
                        <IconButton
                            icon={<Add />}
                            label="Добавить контакт"
                            onClick={(event) => {
                                event.stopPropagation();
                                setLastAction('создание контакта');
                            }}
                        />
                    }
                />
                <p className="text-sm text-content-secondary">Последнее действие: {lastAction}</p>
            </div>
        </div>
    );
}
