'use client';

import type { Dispatch, SetStateAction } from 'react';
import { useDraftStore } from '@/shared/lib/hooks/useDraftStore';
import type { PaymentType } from '@/entities/payment';

/**
 * Черновик визарда создания операции (#570): переживает перезагрузку
 * страницы и закрытие визарда — решение владельца 08.09. Один черновик на
 * все точки входа: направление, название, категория, сумма. propertyId —
 * только выбор шага «Выбрать объект» глобального входа; у входа с объекта
 * объект известен из маршрута и в черновик не пишется. Обёртка владеет
 * только ключом, дефолтом и проверкой формы — сам цикл хранения живёт
 * в useDraftStore (канон usePaymentWizardDraft).
 */

export type OperationWizardDraft = {
  /** Доход/расход — сегмент шага суммы; до явного выбора действует
   * пресет точки входа (черновик хранит только явный выбор). */
  readonly type?: PaymentType;
  /** Название операции (шаг 2; пустое — не черновик). */
  readonly title?: string;
  /** Слаг дефолтного каталога (шаг 3). */
  readonly categorySlug?: string;
  /** Сумма в копейках, целая положительная (шаг 1). */
  readonly amountKopecks?: number;
  /** Объект шага «Выбрать объект» (глобальный вход, шаг 4). */
  readonly propertyId?: string;
};

const DEFAULT_DRAFT: OperationWizardDraft = {};

const OPERATION_WIZARD_DRAFT_KEY = 'operation-wizard-draft';

export function useOperationWizardDraft(): {
  readonly draft: OperationWizardDraft;
  readonly isLoaded: boolean;
  readonly setDraft: Dispatch<SetStateAction<OperationWizardDraft>>;
  readonly clearDraft: () => void;
} {
  return useDraftStore<OperationWizardDraft>({
    storageKey: OPERATION_WIZARD_DRAFT_KEY,
    createDefault: () => DEFAULT_DRAFT,
    validate: validateOperationWizardDraft,
    storage: 'local',
  });
}

function isFilledString(value: unknown): value is string {
  return typeof value === 'string' && value.length > 0;
}

function isPositiveInt(value: unknown): value is number {
  return typeof value === 'number' && Number.isInteger(value) && value > 0;
}

/** Форма-проверка persisted payload: неизвестные значения роняют весь
 * черновик (канон платежного визарда), пустые строки и неположительные
 * суммы отбрасываются — мусор из хранилища не всплывает в визарде. */
export function validateOperationWizardDraft(parsed: unknown): OperationWizardDraft {
  if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) return DEFAULT_DRAFT;

  const record = parsed as Record<string, unknown>;

  const type = record.type === 'income' || record.type === 'expense' ? record.type : undefined;
  if (record.type !== undefined && type === undefined) return DEFAULT_DRAFT;

  const title = isFilledString(record.title) ? record.title : undefined;

  const categorySlug = isFilledString(record.categorySlug) ? record.categorySlug : undefined;
  if (record.categorySlug !== undefined && categorySlug === undefined) return DEFAULT_DRAFT;

  const amountKopecks = isPositiveInt(record.amountKopecks) ? record.amountKopecks : undefined;
  if (record.amountKopecks !== undefined && amountKopecks === undefined) return DEFAULT_DRAFT;

  const propertyId = isFilledString(record.propertyId) ? record.propertyId : undefined;
  if (record.propertyId !== undefined && propertyId === undefined) return DEFAULT_DRAFT;

  return {
    ...(type !== undefined && { type }),
    ...(title !== undefined && { title }),
    ...(categorySlug !== undefined && { categorySlug }),
    ...(amountKopecks !== undefined && { amountKopecks }),
    ...(propertyId !== undefined && { propertyId }),
  };
}
