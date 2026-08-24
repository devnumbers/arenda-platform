import { Admin, Resource, type RaRecord } from 'react-admin';
import HistoryIcon from '@mui/icons-material/History';
import HomeWorkIcon from '@mui/icons-material/HomeWork';
import PaymentIcon from '@mui/icons-material/Payment';
import PeopleIcon from '@mui/icons-material/People';
import SellIcon from '@mui/icons-material/Sell';
import { authProvider } from './authProvider';
import { dataProvider } from './dataProvider';
import { Dashboard } from './Dashboard';
import { i18nProvider } from './i18n';
import { LoginPage } from './LoginPage';
import { actionLabel } from './auditLogs';
import { UserList, UserShow } from './users';
import { SubscriptionPaymentList, SubscriptionPaymentShow } from './subscriptionPayments';
import { TariffList } from './tariffs';
import { tariffRepresentation } from './lib/tariffs';
import { PropertyList, PropertyShow } from './properties';
import { AuditLogList, AuditLogShow } from './auditLogs';

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
      <Resource name="propertyContacts" recordRepresentation="name" />
      <Resource name="auditLogs" list={AuditLogList} show={AuditLogShow} icon={HistoryIcon} recordRepresentation={auditLogRepresentation} />
      {/* История переходов подписки (issue #255): только вложенный просмотр в карточке пользователя. */}
      <Resource name="subscriptionTransitions" />
    </Admin>
  );
}
