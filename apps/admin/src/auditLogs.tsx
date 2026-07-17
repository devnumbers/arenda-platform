import {
  Datagrid,
  DateField,
  DateInput,
  FunctionField,
  List,
  SelectField,
  SelectInput,
  Show,
  SimpleShowLayout,
  TextField,
  type RaRecord,
} from 'react-admin';
import {
  ChoiceChipField,
  UserReferenceField,
  auditActionChoices,
  auditActorRoleChoices,
  auditEntityTypeChoices,
} from './fields';

/** Запись журнала действий (AdminAuditLog из apps/backend/api/openapi/openapi.yaml). */
export interface AuditLogRecord extends RaRecord {
  actorId?: string | null;
  actorRole?: string;
  action?: string;
  entityType?: string | null;
  entityId?: string | null;
  context?: Record<string, unknown>;
  requestId?: string | null;
  ip?: string | null;
  createdAt?: string;
}

/** Русская подпись действия; неизвестное действие выводится как есть. */
export const actionLabel = (action: string): string =>
  auditActionChoices.find((choice) => choice.id === action)?.name ?? action;

const actorRoleLabel = (role: string): string =>
  auditActorRoleChoices.find((choice) => choice.id === role)?.name ?? role;

const auditLogFilters = [
  <SelectInput key="action" source="action" label="Действие" choices={auditActionChoices} />,
  <SelectInput key="entity_type" source="entity_type" label="Сущность" choices={auditEntityTypeChoices} />,
  <DateInput key="date_from" source="date_from" label="С даты" />,
  <DateInput key="date_to" source="date_to" label="По дату" />,
];

// Актор: ссылка на пользователя при actorId, иначе подпись роли («Система», «Аноним»).
const actorField = (
  <FunctionField
    source="actorId"
    sortable={false}
    render={(record: AuditLogRecord) =>
      record.actorId ? <UserReferenceField source="actorId" /> : actorRoleLabel(record.actorRole ?? '')
    }
  />
);

// Действие: русская подпись из реестра auditActionChoices.
const actionField = (
  <FunctionField
    source="action"
    sortable={false}
    render={(record: AuditLogRecord) => actionLabel(record.action ?? '')}
  />
);

// Контекст: pretty-JSON; в context только whitelist-поля без PII (ADR 0020).
const contextField = (
  <FunctionField
    source="context"
    render={(record: AuditLogRecord) => <pre>{JSON.stringify(record.context ?? {}, null, 2)}</pre>}
  />
);

// Колонки журнала — общие для AuditLogList и вкладки «Журнал действий» на UserShow.
// sortable={false} проставлен всем колонкам, кроме createdAt — единственного поля
// из whitelist сортировки бэкенда (auditLogs: ['createdAt'] в dataProvider).
export const AuditLogDatagrid = () => (
  <Datagrid rowClick="show" bulkActionButtons={false}>
    <DateField source="createdAt" showTime />
    {actorField}
    <ChoiceChipField source="actorRole" choices={auditActorRoleChoices} sortable={false} />
    {actionField}
    <SelectField source="entityType" choices={auditEntityTypeChoices} sortable={false} />
    <TextField source="entityId" sortable={false} />
  </Datagrid>
);

export const AuditLogList = () => (
  <List filters={auditLogFilters} sort={{ field: 'createdAt', order: 'DESC' }}>
    <AuditLogDatagrid />
  </List>
);

export const AuditLogShow = () => (
  <Show>
    <SimpleShowLayout>
      <DateField source="createdAt" showTime />
      {actorField}
      <ChoiceChipField source="actorRole" choices={auditActorRoleChoices} />
      {actionField}
      <SelectField source="entityType" choices={auditEntityTypeChoices} />
      <TextField source="entityId" />
      {contextField}
      <TextField source="requestId" />
      <TextField source="ip" />
      <TextField source="id" />
    </SimpleShowLayout>
  </Show>
);
