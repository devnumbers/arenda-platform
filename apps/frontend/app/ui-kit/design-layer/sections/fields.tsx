'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import { ChangeVertical } from '@/shared/assets/icons';
import {
    amountKopecks,
    AmountField,
    Button,
    Checkbox,
    ChipButton,
    RadioGroup,
    RadioGroupItem,
    SearchField,
    Switch,
    TextField,
    Textarea,
} from '@/shared/ui/design';
import styles from '../../page.module.css';

export function TextFieldTitleOutSection(): JSX.Element {
    const [nameValue, setNameValue] = useState('');
    const [nameInValue, setNameInValue] = useState('');
    const [limitedValue, setLimitedValue] = useState('Страхование квартиры');

    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>TextField · Title Out</h3>
            <div className={styles.textFields}>
                <TextField
                    title="Название платежа"
                    required
                    placeholder="Название платежа"
                    description="Необязательно"
                    maxLength={256}
                    value={nameValue}
                    onChange={(event) => setNameValue(event.target.value)}
                    onClear={() => setNameValue('')}
                />
                <TextField
                    title="Название платежа"
                    error="Ошибка"
                    maxLength={256}
                    value={nameInValue}
                    onChange={(event) => setNameInValue(event.target.value)}
                    onClear={() => setNameInValue('')}
                />
                <TextField
                    title="Название платежа"
                    maxLength={32}
                    value={limitedValue}
                    onChange={(event) => setLimitedValue(event.target.value)}
                    onClear={() => setLimitedValue('')}
                />
                <TextField title="Название платежа" placeholder="Название платежа" disabled />
            </div>
        </div>
    );
}

export function TextFieldTitleInSection(): JSX.Element {
    const [titleInValue, setTitleInValue] = useState('');
    const [titleInFilled, setTitleInFilled] = useState('Аренда, январь');
    const [titleInError, setTitleInError] = useState('Страховка');

    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>TextField · Title In (плавающий лейбл)</h3>
            <div className={styles.textFields}>
                <TextField
                    variant="titleIn"
                    title="Название платежа"
                    required
                    maxLength={256}
                    value={titleInValue}
                    onChange={(event) => setTitleInValue(event.target.value)}
                    onClear={() => setTitleInValue('')}
                />
                <TextField
                    variant="titleIn"
                    title="Название платежа"
                    maxLength={256}
                    value={titleInFilled}
                    onChange={(event) => setTitleInFilled(event.target.value)}
                    onClear={() => setTitleInFilled('')}
                />
                <TextField
                    variant="titleIn"
                    title="Название платежа"
                    error="Ошибка"
                    maxLength={256}
                    value={titleInError}
                    onChange={(event) => setTitleInError(event.target.value)}
                    onClear={() => setTitleInError('')}
                />
                <TextField variant="titleIn" title="Название платежа" disabled />
            </div>
        </div>
    );
}

export function CheckboxRadioSwitchSection(): JSX.Element {
    const [checked, setChecked] = useState(true);
    const [unchecked, setUnchecked] = useState(false);
    const [radio, setRadio] = useState('payments');
    const [autoPay, setAutoPay] = useState(true);
    const [paused, setPaused] = useState(false);

    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>Checkbox · Radio · Switch</h3>
            <div className={styles.grid}>
                <div className={styles.links}>
                    <Checkbox
                        id="dl-checkbox-on"
                        checked={checked}
                        onCheckedChange={(value) => setChecked(value === true)}
                    />
                    <label htmlFor="dl-checkbox-on">Автоплатёж включён</label>
                </div>
                <div className={styles.links}>
                    <Checkbox
                        id="dl-checkbox-off"
                        checked={unchecked}
                        onCheckedChange={(value) => setUnchecked(value === true)}
                    />
                    <label htmlFor="dl-checkbox-off">Не выбран</label>
                </div>
                <div className={styles.links}>
                    {/* Частичный выбор (#711): checked="indeterminate" —
                        синий квадрат с белым минусом (мастер-чекбокс
                        группы фильтров истории). */}
                    <Checkbox id="dl-checkbox-partial" checked="indeterminate" onCheckedChange={() => {}} />
                    <label htmlFor="dl-checkbox-partial">Частично (2/4)</label>
                </div>
                <div className={styles.links}>
                    {/* Disabled выбранный (#712): серый квадрат #D3D7D9
                        с белой галочкой (прибитый участник в шите
                        «Действий участника», макет 2184-92510). */}
                    <Checkbox id="dl-checkbox-disabled" checked disabled />
                    <label htmlFor="dl-checkbox-disabled">Недоступен</label>
                </div>
                <RadioGroup value={radio} onValueChange={setRadio}>
                    <div className={styles.links}>
                        <RadioGroupItem id="dl-radio-payments" value="payments" />
                        <label htmlFor="dl-radio-payments">Платежи</label>
                    </div>
                    <div className={styles.links}>
                        <RadioGroupItem id="dl-radio-auto" value="auto" />
                        <label htmlFor="dl-radio-auto">Автоплатежи</label>
                    </div>
                </RadioGroup>
                <div className={styles.links}>
                    <Switch id="dl-switch-auto" checked={autoPay} onCheckedChange={setAutoPay} />
                    <label htmlFor="dl-switch-auto">Автоплатёж</label>
                </div>
                <div className={styles.links}>
                    <Switch id="dl-switch-pause" checked={paused} onCheckedChange={setPaused} />
                    <label htmlFor="dl-switch-pause">Пауза</label>
                </div>
            </div>
        </div>
    );
}

type SearchFieldSectionProps = {
    readonly value: string;
    /** Демо-значение шарится между секциями — живёт на сборке (#820). */
    readonly onValueChange: (value: string) => void;
};

export function SearchFieldSection({ value, onValueChange }: SearchFieldSectionProps): JSX.Element {
    const [searchFilled, setSearchFilled] = useState('аренд');

    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>SearchField</h3>
            <div className={styles.column} style={{ maxWidth: 420 }}>
                <SearchField
                    placeholder="Поиск операций"
                    value={value}
                    onChange={(event) => onValueChange(event.target.value)}
                    onClear={() => onValueChange('')}
                />
                <SearchField
                    placeholder="Поиск операций"
                    value={searchFilled}
                    onChange={(event) => setSearchFilled(event.target.value)}
                    onClear={() => setSearchFilled('')}
                />
            </div>
        </div>
    );
}

export function AmountFieldSection(): JSX.Element {
    const [amount, setAmount] = useState('');
    const [direction, setDirection] = useState<'income' | 'expense'>('income');

    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>AmountField · ввод суммы</h3>
            <p className={styles.groupTitle}>
                Ввод с клавиатуры, только цифры и запятая (маска до 9 999 999,99 ₽); под суммой
                — кнопка направления: клик меняет её значение, без всплывающих окон
                (Figma 835:19795 «Расход»; чип «Формы оплаты» снесён — карта #1005).
                Кнопка заблокирована, пока сумма не введена. Вся область дисплея
                «0 ₽» — цель тапа: инпут расширен за текст измерителя, тап в любом
                месте дисплея фокусирует поле и поднимает клавиатуру (#1151).
            </p>
            <div className={styles.column} style={{ maxWidth: 560 }}>
                <AmountField value={amount} onChange={setAmount} label="Сумма" />
                <div className="flex justify-center gap-2">
                    <ChipButton
                        trailingIcon={<ChangeVertical />}
                        aria-label={`Направление: ${direction === 'income' ? 'Доход' : 'Расход'}. Нажмите, чтобы переключить`}
                        onClick={() => setDirection((prev) => (prev === 'income' ? 'expense' : 'income'))}
                    >
                        {direction === 'income' ? 'Доход' : 'Расход'}
                    </ChipButton>
                </div>
                <Button disabled={amountKopecks(amount) === 0}>Создать платеж</Button>
            </div>
        </div>
    );
}

export function TextareaSection(): JSX.Element {
    const [note, setNote] = useState('');
    const [noteError, setNoteError] = useState('Страховка не оформлена');

    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>Textarea · многострочное поле (#505)</h3>
            <p className={styles.groupTitle}>
                Shell как у TextField (Title Out), бокс 92px — четыре строки 16/18 с
                прокруткой сверх; счётчик «длина/лимит» краснеет на лимите (Figma
                1281:48439, «Заметка» формы контакта).
            </p>
            <div className={styles.textFields}>
                <Textarea
                    title="Заметка"
                    placeholder="Заметка о контакте"
                    description="Необязательно"
                    maxLength={1024}
                    value={note}
                    onChange={(event) => setNote(event.target.value)}
                />
                <Textarea
                    title="Заметка"
                    error="Ошибка"
                    maxLength={1024}
                    value={noteError}
                    onChange={(event) => setNoteError(event.target.value)}
                />
                <Textarea title="Заметка" placeholder="Заметка о контакте" disabled />
            </div>
        </div>
    );
}
