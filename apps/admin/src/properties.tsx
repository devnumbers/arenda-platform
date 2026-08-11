import Box from '@mui/material/Box';
import Typography from '@mui/material/Typography';
import {
  ArrayField,
  Datagrid,
  DateField,
  ImageField,
  List,
  ReferenceManyField,
  SelectField,
  SelectInput,
  Show,
  SingleFieldList,
  Tab,
  TabbedShowLayout,
  TextField,
  TextInput,
  useRecordContext,
} from 'react-admin';
import {
  ChoiceChipField,
  OwnerLinkField,
  UserReferenceField,
  occupancyChoices,
  propertyStatusChoices,
  propertyStatusFilterChoices,
  propertyTypeChoices,
} from './fields';
import { LeaseDatagrid } from './leases';
import { OperationDatagrid } from './operations';
import {
  formatAttributesForCardGrouped,
  isPropertyType,
  type PropertyAttributes,
} from './lib/propertyAttributes';

const propertyFilters = [
  <TextInput key="q" source="q" label="Поиск" alwaysOn />,
  <SelectInput key="status" source="status" label="Статус" choices={propertyStatusFilterChoices} />,
];

// sortable={false} проставлен всем колонкам вне whitelist сортировки бэкенда
// (properties: name, createdAt, updatedAt, status) — иначе бэкенд отвечает 400.
export const PropertyList = () => (
  <List filters={propertyFilters} sort={{ field: 'name', order: 'ASC' }}>
    <Datagrid rowClick="show" bulkActionButtons={false}>
      <TextField source="name" />
      <TextField source="address" sortable={false} />
      <SelectField source="type" choices={propertyTypeChoices} sortable={false} />
      <ChoiceChipField source="status" choices={propertyStatusChoices} />
      <SelectField source="occupancy" choices={occupancyChoices} sortable={false} />
      <OwnerLinkField source="ownerId" sortable={false} />
    </Datagrid>
  </List>
);

interface PropertyRecord {
  type?: unknown;
  attributes?: unknown;
}

/**
 * Read-only блок «Характеристики» для show-вью объекта (issue #133).
 * Форматирует attributes через самодостаточную копию логики кабинета
 * (apps/admin/src/lib/propertyAttributes.ts). Пустые поля скрываются,
 * неизвестный тип объекта → ничего не рендерится.
 */
export const PropertyAttributesBlock = () => {
  const record = useRecordContext<PropertyRecord>();
  if (!record) {
    return null;
  }
  if (!isPropertyType(record.type)) {
    return null;
  }
  const attributes =
    record.attributes && typeof record.attributes === 'object'
      ? (record.attributes as PropertyAttributes)
      : {};
  const groups = formatAttributesForCardGrouped(record.type, attributes);
  if (groups.length === 0) {
    return null;
  }
  return (
    <Box sx={{ mt: 2, mb: 1 }}>
      <Typography variant="h6" gutterBottom>
        Характеристики
      </Typography>
      {groups.map((group, groupIndex) => (
        <Box key={groupIndex} sx={groupIndex > 0 ? { mt: 2 } : undefined}>
          {group.label !== null && (
            <Typography variant="overline" color="text.secondary">
              {group.label}
            </Typography>
          )}
          {group.items.map((item, itemIndex) => (
            <Box
              key={item.label}
              sx={{
                display: 'flex',
                justifyContent: 'space-between',
                gap: 2,
                py: 0.5,
                alignItems: 'baseline',
                ...(itemIndex > 0 ? { borderTop: 1, borderColor: 'divider' } : {}),
              }}
            >
              <Typography variant="body2" color="text.secondary">
                {item.label}
              </Typography>
              <Typography variant="body2" color="text.primary" sx={{ textAlign: 'right' }}>
                {item.value}
              </Typography>
            </Box>
          ))}
        </Box>
      ))}
    </Box>
  );
};

export const PropertyShow = () => (
  <Show>
    <TabbedShowLayout>
      <Tab label="Объект">
        <UserReferenceField source="ownerId" />
        <TextField source="name" />
        <SelectField source="type" choices={propertyTypeChoices} />
        <SelectField source="status" choices={propertyStatusChoices} />
        <SelectField source="occupancy" choices={occupancyChoices} />
        <TextField source="address" />
        <TextField source="description" />
        <ArrayField source="photos">
          <SingleFieldList linkType={false}>
            <ImageField source="url" />
          </SingleFieldList>
        </ArrayField>
        <PropertyAttributesBlock />
        <DateField source="createdAt" showTime />
        <DateField source="updatedAt" showTime />
        <TextField source="id" />
      </Tab>
      <Tab label="Договоры">
        <ReferenceManyField
          reference="leases"
          target="property_id"
          label={false}
          sort={{ field: 'updatedAt', order: 'DESC' }}
        >
          <LeaseDatagrid />
        </ReferenceManyField>
      </Tab>
      <Tab label="Операции">
        <ReferenceManyField
          reference="operations"
          target="property_id"
          label={false}
          sort={{ field: 'operationDate', order: 'DESC' }}
        >
          <OperationDatagrid />
        </ReferenceManyField>
      </Tab>
      <Tab label="Контакты">
        <ReferenceManyField
          reference="propertyContacts"
          target="property_id"
          label={false}
          sort={{ field: 'createdAt', order: 'ASC' }}
        >
          <Datagrid bulkActionButtons={false}>
            <TextField source="name" label="Контакт" sortable={false} />
            <TextField source="phone" label="Телефон" sortable={false} />
            <DateField source="updatedAt" label="Обновлён" showTime sortable={false} />
          </Datagrid>
        </ReferenceManyField>
      </Tab>
    </TabbedShowLayout>
  </Show>
);
