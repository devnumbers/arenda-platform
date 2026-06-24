import type { Metadata } from 'next';
import { PropertyCreateForm } from '@/widgets/properties/ui/PropertyCreateForm';

export const metadata: Metadata = {
  title: 'Создать объект — Arenda Platform',
  description: 'Добавление нового объекта недвижимости',
};

export default function PropertiesNewPage() {
  return (
    <div>
      <h1>Создать объект</h1>
      <PropertyCreateForm />
    </div>
  );
}
