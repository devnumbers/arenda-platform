'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import {
    ArrowDown,
    ArrowLeft,
    BoldPerson,
    BoldSofa,
    Edit,
    Home,
    Move,
    Search,
    SortingDown,
    Star,
    StarOff,
} from '@/shared/assets/icons';
import {
    Button,
    Checkbox,
    ChipButton,
    IconButton,
    ListRow,
    Modal,
    ModalClose,
    ModalContent,
    ModalTrigger,
    PageContent,
    RadioGroup,
    RadioGroupItem,
    SearchField,
    StatusIcon,
    StepsChip,
    StickyBottomBar,
    Switch,
    TextField,
    TopNav,
    TopNavTitle,
} from '@/shared/ui/design';
import styles from './page.module.css';

const dlButtonVariants = ['primary', 'secondary', 'danger', 'clear'] as const;
const dlIconVariants = ['primary', 'secondary', 'danger'] as const;
const statusGradations = ['danger', 'warning', 'good', 'check', 'info'] as const;

/** Витрина дизайн-слоя (ADR 0050, тикет #455): шадкн/ui поверх Radix,
 * Tailwind на токенах, шрифт Onest. Внешний вид сверен с экспортами
 * Figma-фреймов «Рентли. Новые экраны сервиса» (node-id — резолюция #449). */
export function DesignLayerShowcase(): JSX.Element {
    const [nameValue, setNameValue] = useState('');
    const [titleInValue, setTitleInValue] = useState('');
    const [limitedValue, setLimitedValue] = useState('Страхование квартиры');
    const [searchValue, setSearchValue] = useState('');
    const [searchFilled, setSearchFilled] = useState('аренд');
    const [checked, setChecked] = useState(true);
    const [unchecked, setUnchecked] = useState(false);
    const [radio, setRadio] = useState('payments');
    const [autoPay, setAutoPay] = useState(true);
    const [paused, setPaused] = useState(false);
    const [chip, setChip] = useState('name');

    return (
        <>
            <section className={styles.section}>
                <h2 className={styles.sectionTitle}>Дизайн-слой платежей (ADR 0050)</h2>
                <p className={styles.groupTitle}>
                    shadcn/ui поверх Radix · Tailwind на токенах · шрифт Onest
                </p>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>Button</h3>
                    <div className={styles.grid}>
                        {dlButtonVariants.map((variant) => (
                            <Button key={variant} variant={variant}>
                                {variant}
                            </Button>
                        ))}
                        <Button loading>loading</Button>
                        <Button disabled>disabled</Button>
                        <Button variant="primary" leadingIcon={<Home />} trailingIcon={<Edit />}>
                            with icons
                        </Button>
                    </div>
                    <div className={styles.grid}>
                        {dlButtonVariants.map((variant) => (
                            <Button key={variant} variant={variant} size="small">
                                small
                            </Button>
                        ))}
                        <Button variant="secondary" size="small" selected>
                            selected
                        </Button>
                        <Button size="small" loading>
                            loading
                        </Button>
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>IconButton</h3>
                    <div className={styles.grid}>
                        {dlIconVariants.map((variant) => (
                            <IconButton key={variant} variant={variant} icon={<ArrowLeft />} label={`Назад ${variant}`} />
                        ))}
                        <IconButton variant="primary" icon={<Edit />} label="Редактировать" disabled />
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>TextField</h3>
                    <div className={styles.textFields}>
                        <TextField
                            title="Название платежа"
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
                            value={limitedValue}
                            onChange={(event) => setLimitedValue(event.target.value)}
                            onClear={() => setLimitedValue('')}
                        />
                        <TextField
                            title="Название платежа"
                            maxLength={32}
                            value={limitedValue}
                            onChange={(event) => setLimitedValue(event.target.value)}
                            onClear={() => setLimitedValue('')}
                        />
                        <TextField
                            variant="titleIn"
                            title="Название платежа"
                            maxLength={256}
                            value={titleInValue}
                            onChange={(event) => setTitleInValue(event.target.value)}
                            onClear={() => setTitleInValue('')}
                        />
                    </div>
                </div>

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

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>SearchField</h3>
                    <div className={styles.column} style={{ maxWidth: 420 }}>
                        <SearchField
                            placeholder="Поиск операций"
                            value={searchValue}
                            onChange={(event) => setSearchValue(event.target.value)}
                            onClear={() => setSearchValue('')}
                        />
                        <SearchField
                            placeholder="Поиск операций"
                            value={searchFilled}
                            onChange={(event) => setSearchFilled(event.target.value)}
                            onClear={() => setSearchFilled('')}
                        />
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>ChipButton</h3>
                    <div className={styles.grid}>
                        <ChipButton
                            leadingIcon={<SortingDown />}
                            trailingIcon={<ArrowDown />}
                            selected={chip === 'name'}
                            onClick={() => setChip('name')}
                        >
                            По названию
                        </ChipButton>
                        <ChipButton
                            leadingIcon={<SortingDown />}
                            trailingIcon={<ArrowDown />}
                            selected={chip === 'amount'}
                            onClick={() => setChip('amount')}
                        >
                            По сумме
                        </ChipButton>
                        <ChipButton trailingIcon={<ArrowDown />} disabled>
                            disabled
                        </ChipButton>
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>StepsChip</h3>
                    <div className={styles.grid}>
                        <StepsChip step={1} total={5} />
                        <StepsChip step={2} total={6} size="m" />
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>StatusIcon</h3>
                    <div className={styles.grid}>
                        {statusGradations.map((status) => (
                            <StatusIcon key={status} status={status} />
                        ))}
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>ListRow</h3>
                    <div className={styles.column} style={{ maxWidth: 480 }}>
                        <ListRow
                            leading={
                                <span className="flex h-11 w-11 items-center justify-center rounded-pill bg-[#FF8904] text-white shadow-[0_0_0_2.5px_#ffffff]">
                                    <BoldSofa className="h-6 w-6" />
                                </span>
                            }
                            title="Аренда стола"
                            subtitle="Моя квартира"
                            subtitleIcon={<Star />}
                            value="2 500 ₽"
                            description="11 сентября"
                            onSelect={() => undefined}
                        />
                        <ListRow
                            leading={
                                <span className="flex h-11 w-11 items-center justify-center rounded-pill bg-surface-muted text-content">
                                    <BoldSofa className="h-6 w-6" />
                                </span>
                            }
                            title="Аренда стола"
                            subtitle="Моя квартира"
                            subtitleIcon={<StarOff />}
                            value="2 500 ₽"
                            description="11 сентября"
                            trailing={
                                <span className={styles.links}>
                                    <IconButton icon={<StarOff />} label="В избранное" />
                                    <IconButton icon={<Move />} label="Переставить" />
                                </span>
                            }
                        />
                        <ListRow
                            leading={<StatusIcon status="danger" />}
                            title="Просроченный платёж"
                            subtitle="2 дня"
                            value="32 000 ₽"
                        />
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>TopNav</h3>
                    <div className={styles.column} style={{ maxWidth: 480 }}>
                        <TopNav
                            leading={
                                <span className="px-5 text-lg font-bold text-primary">Рентли</span>
                            }
                            trailing={
                                <span className="flex items-center gap-3 pr-2">
                                    <span className="text-sm font-medium text-content">Даниил</span>
                                    <span className="flex h-11 w-11 items-center justify-center rounded-pill bg-surface-muted text-content">
                                        <BoldPerson className="h-6 w-6" />
                                    </span>
                                </span>
                            }
                        />
                        <TopNav leading={<IconButton icon={<ArrowLeft />} label="Назад" />}>
                            <TopNavTitle title="Избранные платежи" subtitle="На паузе" />
                        </TopNav>
                        <TopNav
                            leading={<IconButton icon={<ArrowLeft />} label="Назад" />}
                            trailing={<IconButton icon={<Search />} label="Поиск" />}
                        >
                            <StepsChip step={1} total={5} size="m" />
                        </TopNav>
                        <TopNav leading={<IconButton icon={<ArrowLeft />} label="Назад" />}>
                            <div className="w-full">
                                <SearchField
                                    placeholder="Поиск операций"
                                    value={searchValue}
                                    onChange={(event) => setSearchValue(event.target.value)}
                                    onClear={() => setSearchValue('')}
                                />
                            </div>
                        </TopNav>
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>PageContent</h3>
                    <div className="rounded-button bg-surface-muted p-4">
                        <PageContent className="pt-4 pb-4">
                            <p className="text-sm text-content-secondary">
                                Центрированная колонка max-width 560 — контейнер новых экранов
                                (отступы 72/136 уменьшены для витрины).
                            </p>
                        </PageContent>
                    </div>
                </div>

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

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>StickyBottomBar</h3>
                    <p className={styles.groupTitle}>
                        Живой образец закреплён внизу окна — вариант из визарда (кнопка «Далее»).
                    </p>
                </div>
            </section>

            <StickyBottomBar>
                <Button>Далее</Button>
            </StickyBottomBar>
        </>
    );
}
