import { type MouseEvent } from 'react';
import Chip from '@mui/material/Chip';
import {
  FunctionField,
  Link,
  ReferenceField,
  TextField,
  useRecordContext,
  type FieldProps,
  type RaRecord,
  type ReferenceFieldProps,
} from 'react-admin';

// Значения enum-полей синхронизированы с apps/backend/api/openapi/openapi.yaml.

interface Choice {
  id: string;
  name: string;
}

export const roleChoices: Choice[] = [
  { id: 'owner', name: 'Собственник' },
  { id: 'admin', name: 'Администратор' },
];

export const subscriptionStatusChoices: Choice[] = [
  { id: 'active', name: 'Активна' },
  { id: 'grace', name: 'Грейс-период' },
  { id: 'cancelled', name: 'Отменена' },
];

export const subscriptionPaymentStatusChoices: Choice[] = [
  { id: 'pending', name: 'Ожидает оплаты' },
  { id: 'succeeded', name: 'Оплачен' },
  { id: 'failed', name: 'Ошибка оплаты' },
  { id: 'refunded', name: 'Возвращён' },
  { id: 'partial_refunded', name: 'Частичный возврат' },
  { id: 'refunding', name: 'Возврат выполняется' },
];

export const subscriptionPaymentPeriodChoices: Choice[] = [
  { id: 'month', name: 'Месяц' },
  { id: 'year', name: 'Год' },
];

export const propertyTypeChoices: Choice[] = [
  { id: 'apartment', name: 'Квартира' },
  { id: 'room', name: 'Комната' },
  { id: 'apartments', name: 'Апартаменты' },
  { id: 'house', name: 'Дом' },
  { id: 'commercial', name: 'Коммерческое' },
  { id: 'office', name: 'Офис' },
  { id: 'warehouse', name: 'Склад' },
  { id: 'garage', name: 'Гараж' },
  { id: 'parking', name: 'Парковка' },
  { id: 'land', name: 'Земельный участок' },
];

export const propertyStatusChoices: Choice[] = [
  { id: 'active', name: 'Активен' },
  { id: 'maintenance', name: 'На обслуживании' },
  { id: 'archived', name: 'В архиве' },
];

// Choices для фильтра status списка объектов: query-enum эндпоинтов списка
// (GET /admin/properties, GET /admin/users/{id}/properties) допускает только
// active/all/archived, maintenance в фильтре недоступен.
export const propertyStatusFilterChoices: Choice[] = [
  { id: 'active', name: 'Активен' },
  { id: 'archived', name: 'В архиве' },
];

export const occupancyChoices: Choice[] = [
  { id: 'free', name: 'Свободен' },
  { id: 'occupied', name: 'Занят' },
];

export const leaseStatusChoices: Choice[] = [
  { id: 'awaiting_start', name: 'Ожидает начала' },
  { id: 'active', name: 'Активен' },
  { id: 'requires_action', name: 'Требует действия' },
  { id: 'completed', name: 'Завершён' },
  { id: 'archived', name: 'В архиве' },
];

export const operationTypeChoices: Choice[] = [
  { id: 'income', name: 'Доход' },
  { id: 'expense', name: 'Расход' },
];

export const operationStatusChoices: Choice[] = [
  { id: 'pending', name: 'Ожидает' },
  { id: 'overdue', name: 'Просрочена' },
  { id: 'paid', name: 'Оплачена' },
  { id: 'received', name: 'Получена' },
];

interface PersonName {
  name?: string | null;
  surname?: string | null;
  patronymic?: string | null;
}

/** ФИО в формате «Фамилия Имя Отчество», пустые части пропускаются. */
export const fullName = (person: PersonName): string =>
  [person.surname, person.name, person.patronymic].filter(Boolean).join(' ');

export const asPersonName = (value: unknown): PersonName => (value && typeof value === 'object' ? (value as PersonName) : {});

const moneyFormatter = new Intl.NumberFormat('ru-RU', { style: 'currency', currency: 'RUB' });

/** Форматирует сумму в копейках как рубли (например «1 500,00 ₽»). */
export const formatKopecks = (kopecks: number): string => moneyFormatter.format(kopecks / 100);

const stopPropagation = (event: MouseEvent<HTMLElement>) => event.stopPropagation();

/** ФИО из частей name/surname/patronymic текущей записи. */
export const FullNameField = (props: FieldProps) => (
  <FunctionField {...props} render={(record) => fullName(asPersonName(record)) || null} />
);

/** Денежное поле: значение в копейках выводится в рублях. */
export const MoneyField = (props: FieldProps) => (
  <FunctionField
    {...props}
    render={(record) => {
      const value: unknown = record[props.source];
      return typeof value === 'number' ? formatKopecks(value) : null;
    }}
  />
);

/** Значение enum-поля в виде чипа с русской подписью из choices. */
export const ChoiceChipField = ({ choices, ...props }: FieldProps & { choices: Choice[] }) => {
  const record = useRecordContext();
  if (!record) {
    return null;
  }
  const value: unknown = record[props.source];
  if (value === null || value === undefined || value === '') {
    return null;
  }
  const choice = choices.find((item) => item.id === value);
  return <Chip size="small" label={choice ? choice.name : String(value)} />;
};

/**
 * Ссылка на Show пользователя: телефон (+ ФИО, если есть в записи).
 * source указывает на поле с id пользователя (userId/ownerId); запросов не выполняет,
 * используются данные текущей записи (userPhone/phone).
 */
export const UserLinkField = (props: FieldProps) => {
  const record = useRecordContext();
  if (!record) {
    return null;
  }
  const userId: unknown = record[props.source];
  if (typeof userId !== 'string' || userId === '') {
    return null;
  }
  const phone =
    typeof record.userPhone === 'string' ? record.userPhone : typeof record.phone === 'string' ? record.phone : '';
  const name = fullName(asPersonName(record));
  return (
    <Link to={`/users/${userId}/show`} onClick={stopPropagation}>
      {name ? `${phone} (${name})` : phone || userId}
    </Link>
  );
};

/**
 * Владелец записи: ссылка на Show пользователя, текст — ownerPhone.
 * source указывает на поле ownerId; запросов не выполняет,
 * используются ownerPhone/ownerId текущей записи (AdminProperty, AdminTenantContact).
 */
export const OwnerLinkField = (props: FieldProps) => {
  const record = useRecordContext();
  if (!record) {
    return null;
  }
  const ownerId: unknown = record[props.source];
  if (typeof ownerId !== 'string' || ownerId === '') {
    return null;
  }
  const phone = typeof record.ownerPhone === 'string' ? record.ownerPhone : '';
  return (
    <Link to={`/users/${ownerId}/show`} onClick={stopPropagation}>
      {phone || ownerId}
    </Link>
  );
};

interface TenantContactValue extends PersonName {
  phone?: string | null;
}

/** Вложенный tenantContact договора: «Фамилия Имя Отчество, телефон». */
export const TenantContactField = (props: FieldProps) => (
  <FunctionField
    {...props}
    render={(record) => {
      const contact: unknown = record[props.source];
      if (!contact || typeof contact !== 'object') {
        return null;
      }
      const value = contact as TenantContactValue;
      const name = fullName(value);
      const phone = typeof value.phone === 'string' ? value.phone : '';
      if (name && phone) {
        return `${name}, ${phone}`;
      }
      return name || phone || null;
    }}
  />
);

/**
 * Название объекта из текущей записи (source="propertyName") со ссылкой на Show объекта.
 * Запросов не выполняет, использует propertyName/propertyId текущей записи.
 */
export const PropertyLinkField = (props: FieldProps) => {
  const record = useRecordContext();
  if (!record) {
    return null;
  }
  const propertyName: unknown = record[props.source];
  if (typeof propertyName !== 'string' || propertyName === '') {
    return null;
  }
  const propertyId: unknown = record.propertyId;
  if (typeof propertyId !== 'string' || propertyId === '') {
    return <>{propertyName}</>;
  }
  return (
    <Link to={`/properties/${propertyId}/show`} onClick={stopPropagation}>
      {propertyName}
    </Link>
  );
};

// Ссылочные поля — только для Show-страниц (1 запись = 1 запрос getOne).

const renderUser = (user: RaRecord) => {
  const phone = typeof user.phone === 'string' ? user.phone : '';
  const name = fullName(asPersonName(user));
  return name ? `${phone} (${name})` : phone;
};

const renderLease = (lease: RaRecord) => {
  const propertyName = typeof lease.propertyName === 'string' ? lease.propertyName : '';
  const tenantName = fullName(asPersonName(lease.tenantContact));
  return tenantName ? `${propertyName} → ${tenantName}` : propertyName;
};

type ReferenceProps = Omit<ReferenceFieldProps, 'reference' | 'children'>;

/** Владелец/пользователь: ссылка на Show пользователя, телефон (+ ФИО). */
export const UserReferenceField = (props: ReferenceProps) => (
  <ReferenceField reference="users" link="show" {...props}>
    <FunctionField render={renderUser} />
  </ReferenceField>
);

/** Объект недвижимости: ссылка на Show объекта, название. */
export const PropertyReferenceField = (props: ReferenceProps) => (
  <ReferenceField reference="properties" link="show" {...props}>
    <TextField source="name" />
  </ReferenceField>
);

/** Договор аренды: ссылка на Show договора, «{propertyName} → {ФИО арендатора}». */
export const LeaseReferenceField = (props: ReferenceProps) => (
  <ReferenceField reference="leases" link="show" {...props}>
    <FunctionField render={renderLease} />
  </ReferenceField>
);
