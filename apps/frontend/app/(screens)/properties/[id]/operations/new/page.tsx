import type { Metadata } from 'next';
import { OperationCreateWizardScreen } from '@/widgets/payments';
import { operationPresetFromQueryParam } from '@/features/payments';

/**
 * Визард создания операции с объекта (#570): объектного шага нет —
 * категория последний шаг с сабмитом «Добавить операцию» (аннотация
 * Figma 1863:67305). Направление — пресет ?type= от точки входа.
 */

export const metadata: Metadata = {
  title: 'Новая операция — Рентли',
};

type PropertyOperationNewRoutePageProps = {
  params: Promise<{ id: string }>;
  searchParams: Promise<{ [key: string]: string | string[] | undefined }>;
};

export default async function PropertyOperationNewRoutePage({
  params,
  searchParams,
}: PropertyOperationNewRoutePageProps) {
  const { id } = await params;
  const query = await searchParams;

  return (
    <OperationCreateWizardScreen
      mode="property"
      propertyId={id}
      presetType={operationPresetFromQueryParam(query.type)}
    />
  );
}
