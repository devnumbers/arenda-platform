'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import {
    Button,
    ConfirmDialog,
    Modal,
    ModalClose,
    ModalContent,
    ModalTrigger,
    SupportModal,
} from '@/shared/ui/design';
import styles from '../../page.module.css';

export function ModalSection(): JSX.Element {
    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>Modal</h3>
            <div className={styles.grid}>
                <Modal>
                    <ModalTrigger asChild>
                        <Button>Открыть модалку</Button>
                    </ModalTrigger>
                    <ModalContent title="Что создать?" description="Выберите тип регулярного платежа">
                        <div className={styles.column} style={{ alignItems: 'stretch' }}>
                            <Button variant="secondary">Платеж</Button>
                            <Button variant="secondary">Автоплатеж</Button>
                            <ModalClose asChild>
                                <Button variant="clear">Отмена</Button>
                            </ModalClose>
                        </div>
                    </ModalContent>
                </Modal>
            </div>
        </div>
    );
}

export function SupportModalSection(): JSX.Element {
    const [supportOpen, setSupportOpen] = useState(false);

    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>SupportModal · «Связаться с нами»</h3>
            <p className={styles.groupTitle}>
                Модалка шагов входа (28.10): с восстановлением страницы /support
                (макеты 40000074:80636/40000075:80950) осталась только на логине —
                гость не покидает флоу входа. Канва Modal — карточка ≥768 / vaul-шит
                ниже; лого HeaderLogo 112×28, заголовок H1 28/32 + подпись 16/18,
                Primary «Написать в Телеграм» (внешняя ссылка в новой вкладке) и
                Secondary-строка почты с копированием в буфер (Figma 2355:52709 —
                десктоп, 2355:52684 — планшет, 2355:52746 — мобайл). Карточка макета
                уже канона — max-w-400.
            </p>
            <div className={styles.grid}>
                <Button onClick={() => setSupportOpen(true)}>
                    Открыть «Связаться с нами»
                </Button>
            </div>
            <SupportModal open={supportOpen} onOpenChange={setSupportOpen} />
        </div>
    );
}

export function ConfirmDialogSection(): JSX.Element {
    const [confirmOpen, setConfirmOpen] = useState(false);
    const [deleteOpen, setDeleteOpen] = useState(false);
    const [confirmLargeOpen, setConfirmLargeOpen] = useState(false);
    const [deletePropertyOpen, setDeletePropertyOpen] = useState(false);

    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>ConfirmDialog · подтверждение</h3>
            <p className={styles.groupTitle}>
                Карточка на десктопе, нижний шит на мобиле: отмена secondary +
                подтверждение primary; разрушительное — confirmVariant=&quot;danger&quot;.
                Закрытие — на потребителе.
            </p>
            <div className={styles.grid}>
                <Button onClick={() => setConfirmOpen(true)}>Подтвердить действие</Button>
                <Button variant="danger" onClick={() => setDeleteOpen(true)}>
                    Удалить (danger)
                </Button>
                <Button onClick={() => setConfirmLargeOpen(true)}>
                    Подтвердить (описание 16)
                </Button>
                <Button variant="danger" onClick={() => setDeletePropertyOpen(true)}>
                    Удалить объект (столбиком)
                </Button>
            </div>
            <ConfirmDialog
                open={confirmOpen}
                onOpenChange={setConfirmOpen}
                title="Сменить тариф?"
                description="Новые условия применятся со следующего периода"
                confirmLabel="Сменить"
                onConfirm={() => setConfirmOpen(false)}
            />
            <ConfirmDialog
                open={deleteOpen}
                onOpenChange={setDeleteOpen}
                title="Удалить контакт?"
                description="Контакт исчезнет из книги контактов объекта"
                confirmLabel="Удалить"
                confirmVariant="danger"
                onConfirm={() => setDeleteOpen(false)}
            />
            {/* #627: подпись макета R/400 16/18 — descriptionClassName
             * поверх каноничных 14px. */}
            <ConfirmDialog
                open={confirmLargeOpen}
                onOpenChange={setConfirmLargeOpen}
                title="Завершить аренду?"
                description="Объект станет свободным, арендный платеж завершится. Данные аренды сохранятся в разделе «Прошлые аренды»"
                descriptionClassName="text-base leading-[18px]"
                confirmLabel="Завершить"
                cancelLabel="Отменить"
                onConfirm={() => setConfirmLargeOpen(false)}
            />
            {/* #629: stacked + children + titleClassName — кнопки
             * столбиком (danger сверху, решение владельца 12.09),
             * заголовок H1 28/32, красное предупреждение о
             * последствиях (danger-soft) между описанием и кнопками.
             * Figma 1583:56558. */}
            <ConfirmDialog
                open={deletePropertyOpen}
                onOpenChange={setDeletePropertyOpen}
                title="Удалить объект?"
                titleClassName="text-[28px] leading-8"
                description="Объект будет удален. Вместо удаления объект можно перевести в архив"
                descriptionClassName="text-base leading-[18px]"
                confirmLabel="Удалить"
                cancelLabel="Отменить"
                confirmVariant="danger"
                stacked
                onConfirm={() => setDeletePropertyOpen(false)}
            >
                <p className="text-sm leading-4 text-danger-soft">
                    Будут удалены данные аренд объекта, все операции объекта, платежи, контакты и задачи, связанные с объектом. Это действие нельзя отменить
                </p>
            </ConfirmDialog>
        </div>
    );
}
