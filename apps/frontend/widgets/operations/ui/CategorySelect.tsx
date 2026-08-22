'use client';

import {
    useEffect,
    useRef,
    useState,
    type JSX,
    type KeyboardEvent,
} from 'react';
import { Button } from '@/shared/ui/button';
import { Icon } from '@/shared/ui/icon';
import { Select } from '@/shared/ui/select';
import { runListboxAction } from '@/shared/ui/select/listbox-keyboard';
import { Loading } from '@/shared/assets/icons';
import { ApiError } from '@/shared/api/errors';
import { notify } from '@/shared/lib/notifications';
import {
    useCreateOperationCategory,
    useOperationCategories,
} from '@/features/operation-categories';
import selectStyles from '@/shared/ui/select/Select.module.css';
import styles from './CategorySelect.module.css';

const OTHER_LABEL = 'Иное';

export type CategorySelectProps = {
    readonly type: 'income' | 'expense';
    readonly value?: string;
    readonly onChange: (categoryId: string) => void;
    readonly error?: string;
    readonly disabled?: boolean;
    /**
     * Property context of the enclosing operation form. When set, a category
     * created inline is stored in the account of the property's data owner
     * (shared-access members included); when absent, in the actor's own
     * account.
     */
    readonly propertyId?: string;
};

export function CategorySelect({
    type,
    value,
    onChange,
    error,
    disabled,
    propertyId,
}: CategorySelectProps): JSX.Element {
    const categoriesQuery = useOperationCategories(type);
    const createCategory = useCreateOperationCategory();

    const [isOpen, setIsOpen] = useState(false);
    const [isCreating, setIsCreating] = useState(false);
    const [draftName, setDraftName] = useState('');
    const inputRef = useRef<HTMLInputElement>(null);
    // Shared by both shapes of the inline row (option / create form): the li
    // DOM node survives the shape switch, so the isCreating effect can hand
    // focus back to the option after the create form cancels.
    const otherOptionRef = useRef<HTMLLIElement>(null);
    // Synchronous mirrors of isCreating/draftName. Select calls onChange and
    // onOpenChange in the same event tick, before batched state commits, so
    // the close handler must read refs to see a just-cancelled draft.
    const isCreatingRef = useRef(false);
    const draftNameRef = useRef('');

    const label = type === 'income' ? 'Категория дохода' : 'Категория расхода';

    useEffect(() => {
        if (isCreating) {
            inputRef.current?.focus();
        } else {
            // Focus lands after the commit — the create input is gone, the
            // row is an option again. No-op when the dropdown itself is
            // closed (the row is unmounted, the ref is null).
            otherOptionRef.current?.focus();
        }
    }, [isCreating]);

    const startCreating = () => {
        isCreatingRef.current = true;
        setIsCreating(true);
    };

    const cancelCreating = () => {
        isCreatingRef.current = false;
        draftNameRef.current = '';
        setIsCreating(false);
        setDraftName('');
    };

    const submitCreate = () => {
        const name = draftNameRef.current.trim();
        if (name === '' || createCategory.isPending) {
            return;
        }
        createCategory.mutate(
            { type, name, ...(propertyId ? { property_id: propertyId } : {}) },
            {
                onSuccess: (category) => {
                    cancelCreating();
                    onChange(category.id);
                    setIsOpen(false);
                },
                onError: (createFailure) => {
                    if (
                        createFailure instanceof ApiError &&
                        createFailure.status === 409
                    ) {
                        const existing = (categoriesQuery.data ?? []).find(
                            (category) =>
                                category.name.trim().toLowerCase() ===
                                name.toLowerCase(),
                        );
                        if (existing) {
                            cancelCreating();
                            onChange(existing.id);
                            setIsOpen(false);
                            notify.scenarios.operations.categoryAlreadyExists();
                            return;
                        }
                        // Stale cache: revalidate and report the failure.
                        void categoriesQuery.refetch();
                    }
                    notify.scenarios.operations.categoryCreateError();
                },
            },
        );
    };

    const handleCreateKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
        if (event.key === 'Enter') {
            event.preventDefault();
            submitCreate();
        }
        if (event.key === 'Escape') {
            // Cancel the draft, not the dropdown: stop the Escape from
            // bubbling into Select's listbox-level close. Focus returns to
            // the row in the isCreating effect, after the commit.
            event.stopPropagation();
            event.preventDefault();
            cancelCreating();
        }
    };

    if (categoriesQuery.isPending) {
        return (
            <Select
                label={label}
                options={[]}
                onChange={() => {}}
                disabled
                required
                placeholder="Загрузка..."
            />
        );
    }

    if (categoriesQuery.isError) {
        return (
            <div className={styles.errorState}>
                <Select
                    label={label}
                    options={[]}
                    onChange={() => {}}
                    disabled
                    required
                    placeholder="Выберите категорию"
                    error="Не удалось загрузить категории"
                />
                <Button
                    variant="secondary"
                    size="small"
                    loading={categoriesQuery.isFetching}
                    onClick={() => void categoriesQuery.refetch()}
                >
                    Повторить
                </Button>
            </div>
        );
    }

    const options = (categoriesQuery.data ?? []).map((category) => ({
        value: category.id,
        label: category.name,
    }));

    // Roving-focus listbox semantics on the "other" option, same as Select's
    // own options; Enter/Space opens the inline create form. Escape is left
    // to Select's listbox-level handler, which closes and refocuses the
    // trigger for every row, footer rows included.
    const handleOtherKeyDown = (event: KeyboardEvent<HTMLLIElement>) => {
        runListboxAction(event, { onSelect: startCreating });
    };

    // While creating, the row is an inline form, not a listbox option — no
    // role/onClick, so the input keeps its own keyboard contract.
    const otherRow = isCreating ? (
        <li className={selectStyles.listItem} ref={otherOptionRef}>
            <span className={styles.createRow}>
                <span className={styles.createInputRow}>
                    <input
                        ref={inputRef}
                        className={styles.createInput}
                        value={draftName}
                        onChange={(event) => {
                            const nextName = event.currentTarget.value;
                            draftNameRef.current = nextName;
                            setDraftName(nextName);
                        }}
                        onKeyDown={handleCreateKeyDown}
                        disabled={createCategory.isPending}
                        placeholder="Название категории"
                        aria-label="Название новой категории"
                    />
                    {createCategory.isPending && (
                        <Icon size="s" className={styles.spinner}>
                            <Loading />
                        </Icon>
                    )}
                </span>
            </span>
        </li>
    ) : (
        <li
            className={selectStyles.listItem}
            ref={otherOptionRef}
            role="option"
            aria-selected={false}
            tabIndex={-1}
            onClick={startCreating}
            onKeyDown={handleOtherKeyDown}
        >
            {OTHER_LABEL}
        </li>
    );

    return (
        <Select
            label={label}
            value={value}
            options={options}
            onChange={(categoryId) => {
                // Select calls onChange before closing, so the draft is
                // already discarded by the time the close handler runs.
                cancelCreating();
                onChange(categoryId);
            }}
            error={error}
            disabled={disabled}
            required
            placeholder="Выберите категорию"
            open={isOpen}
            onOpenChange={(nextOpen) => {
                setIsOpen(nextOpen);
                if (!nextOpen) {
                    if (
                        isCreatingRef.current &&
                        draftNameRef.current.trim() !== '' &&
                        !createCategory.isPending
                    ) {
                        submitCreate();
                    } else {
                        cancelCreating();
                    }
                }
            }}
            footerRow={otherRow}
        />
    );
}
