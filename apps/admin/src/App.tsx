import { Admin, Resource, type RaRecord } from 'react-admin';
import ContactPageIcon from '@mui/icons-material/ContactPage';
import DescriptionIcon from '@mui/icons-material/Description';
import HistoryIcon from '@mui/icons-material/History';
import HomeWorkIcon from '@mui/icons-material/HomeWork';
import PaymentIcon from '@mui/icons-material/Payment';
import PeopleIcon from '@mui/icons-material/People';
import ReceiptLongIcon from '@mui/icons-material/ReceiptLong';
import SellIcon from '@mui/icons-material/Sell';
import { authProvider } from './authProvider';
import { dataProvider } from './dataProvider';
import { Dashboard } from './Dashboard';
import { i18nProvider } from './i18n';
import { LoginPage } from './LoginPage';
import { asPersonName, fullName } from './fields';
import { UserList, UserShow } from './users';
import { SubscriptionPaymentList, SubscriptionPaymentShow } from './subscriptionPayments';
import { TariffList } from './tariffs';
import { tariffRepresentation } from './lib/tariffs';
import { PropertyList, PropertyShow } from './properties';
import { LeaseList, LeaseShow } from './leases';
import { TenantContactList, TenantContactShow } from './tenantContacts';
import { OperationList, OperationShow } from './operations';
import { AuditLogList, AuditLogShow, actionLabel } from './auditLogs';

/** Контакт арендатора: ФИО из частей name/surname/patronymic, иначе #id. */
const tenantContactRepresentation = (record: RaRecord) => fullName(asPersonName(record)) || `#${record.id}`;

/** Договор: «{propertyName} → {ФИО арендатора}»; без арендатора — propertyName, иначе #id. */
const leaseRepresentation = (record: RaRecord) => {
  const propertyName = typeof record.propertyName === 'string' ? record.propertyName : '';
  const tenantName = fullName(asPersonName(record.tenantContact));
  const label = tenantName ? `${propertyName} → ${tenantName}` : propertyName;
  return label || `#${record.id}`;
};

/** Запись журнала: русская подпись действия (неизвестное — как есть), иначе #id. */
const auditLogRepresentation = (record: RaRecord) =>
  (typeof record.action === 'string' && record.action !== '' ? actionLabel(record.action) : '') || `#${record.id}`;

export default function App() {
  return (
    <Admin
      authProvider={authProvider}
      dataProvider={dataProvider}
      i18nProvider={i18nProvider}
      loginPage={LoginPage}
      dashboard={Dashboard}
    >
      <Resource name="users" list={UserList} show={UserShow} icon={PeopleIcon} recordRepresentation="phone" />
      <Resource name="subscriptionPayments" list={SubscriptionPaymentList} show={SubscriptionPaymentShow} icon={PaymentIcon} recordRepresentation="userPhone" />
      <Resource name="tariffs" list={TariffList} icon={SellIcon} recordRepresentation={tariffRepresentation} />
      <Resource name="properties" list={PropertyList} show={PropertyShow} icon={HomeWorkIcon} recordRepresentation="name" />
      <Resource name="leases" list={LeaseList} show={LeaseShow} icon={DescriptionIcon} recordRepresentation={leaseRepresentation} />
      <Resource name="tenantContacts" list={TenantContactList} show={TenantContactShow} icon={ContactPageIcon} recordRepresentation={tenantContactRepresentation} />
      <Resource name="propertyContacts" recordRepresentation="name" />
      <Resource name="operations" list={OperationList} show={OperationShow} icon={ReceiptLongIcon} recordRepresentation="name" />
      <Resource name="auditLogs" list={AuditLogList} show={AuditLogShow} icon={HistoryIcon} recordRepresentation={auditLogRepresentation} />
    </Admin>
  );
}
