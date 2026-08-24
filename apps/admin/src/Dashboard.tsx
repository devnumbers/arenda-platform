import { useEffect, useState } from 'react';
import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Card from '@mui/material/Card';
import CardContent from '@mui/material/CardContent';
import Chip from '@mui/material/Chip';
import CircularProgress from '@mui/material/CircularProgress';
import Grid from '@mui/material/Grid';
import List from '@mui/material/List';
import ListItem from '@mui/material/ListItem';
import ListItemText from '@mui/material/ListItemText';
import Typography from '@mui/material/Typography';
import { Link, useDataProvider } from 'react-admin';
import { formatKopecks, fullName, subscriptionPaymentStatusChoices } from './fields';
import type { AdminDataProvider } from './dataProvider';

// Ссылка на список пользователей с активной подпиской: react-admin читает фильтры списка
// из query-параметра filter (JSON), ключ совпадает с source фильтра в UserList.
const ACTIVE_SUBSCRIPTIONS_URL = `/users?filter=${encodeURIComponent(JSON.stringify({ subscription_status: 'active' }))}`;

// Форматтеры вынесены на уровень модуля, чтобы не пересоздавать их на каждый рендер.
const countFormatter = new Intl.NumberFormat('ru-RU');
const dateFormatter = new Intl.DateTimeFormat('ru-RU', { dateStyle: 'short', timeStyle: 'short' });

type SubscriptionPaymentStatus = 'pending' | 'succeeded' | 'failed' | 'refunded' | 'partial_refunded' | 'refunding';

// Структуры синхронизированы со схемой AdminStats в apps/backend/api/openapi/openapi.yaml.
interface AdminStatsRecentUser {
  id: string;
  phone: string;
  name?: string | null;
  surname?: string | null;
  createdAt: string;
}

interface AdminStatsRecentPayment {
  id: string;
  userId: string;
  userPhone: string;
  amountKopecks: number;
  status: SubscriptionPaymentStatus;
  createdAt: string;
}

interface AdminStats {
  usersTotal: number;
  usersNewLast30d: number;
  subscriptionsActive: number;
  propertiesActive: number;
  propertiesArchived: number;
  paymentsSucceededTotalKopecksLast30d: number;
  paymentsFailedCountLast30d: number;
  paymentsRefundedCountLast30d: number;
  recentUsers: AdminStatsRecentUser[];
  recentPayments: AdminStatsRecentPayment[];
}

interface StatCardProps {
  title: string;
  value: number;
  to: string;
  subtitle?: string;
}

/** Карточка-счётчик со ссылкой на список ресурса. */
const StatCard = ({ title, value, to, subtitle }: StatCardProps) => (
  <Link to={to} underline="none" sx={{ display: 'block', height: '100%' }}>
    <Card sx={{ height: '100%' }}>
      <CardContent>
        <Typography variant="subtitle2" color="text.secondary">
          {title}
        </Typography>
        <Typography variant="h4">{countFormatter.format(value)}</Typography>
        {subtitle ? (
          <Typography variant="body2" color="text.secondary">
            {subtitle}
          </Typography>
        ) : null}
      </CardContent>
    </Card>
  </Link>
);

/** Чип статуса платежа с русской подписью из subscriptionPaymentStatusChoices. */
const PaymentStatusChip = ({ status }: { status: SubscriptionPaymentStatus }) => {
  const choice = subscriptionPaymentStatusChoices.find((item) => item.id === status);
  return <Chip size="small" label={choice ? choice.name : status} />;
};

/** Блок «Подписки за 30 дней»: оплачено, ошибки, возвраты. */
const PaymentsSummaryCard = ({ stats }: { stats: AdminStats }) => (
  <Card sx={{ height: '100%' }}>
    <CardContent>
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 1 }}>
        <Typography variant="h6">Подписки за 30 дней</Typography>
        <Link to="/subscriptionPayments">Все платежи</Link>
      </Box>
      <Box sx={{ display: 'flex', gap: 4, flexWrap: 'wrap' }}>
        <Box>
          <Typography variant="subtitle2" color="text.secondary">
            Оплачено
          </Typography>
          <Typography variant="h5">{formatKopecks(stats.paymentsSucceededTotalKopecksLast30d)}</Typography>
        </Box>
        <Box>
          <Typography variant="subtitle2" color="text.secondary">
            Ошибок
          </Typography>
          <Typography variant="h5">{countFormatter.format(stats.paymentsFailedCountLast30d)}</Typography>
        </Box>
        <Box>
          <Typography variant="subtitle2" color="text.secondary">
            Возвратов
          </Typography>
          <Typography variant="h5">{countFormatter.format(stats.paymentsRefundedCountLast30d)}</Typography>
        </Box>
      </Box>
    </CardContent>
  </Card>
);

/** Карточка «Последние пользователи»: телефон (+ ФИО), дата регистрации, ссылка на Show. */
const RecentUsersCard = ({ users }: { users: AdminStatsRecentUser[] }) => (
  <Card sx={{ height: '100%' }}>
    <CardContent>
      <Typography variant="h6" gutterBottom>
        Последние пользователи
      </Typography>
      {users.length > 0 ? (
        <List dense disablePadding>
          {users.map((user) => {
            const name = fullName(user);
            return (
              <ListItem key={user.id} disableGutters>
                <ListItemText
                  primary={
                    <Link to={`/users/${user.id}/show`}>
                      {name ? `${user.phone} (${name})` : user.phone}
                    </Link>
                  }
                  secondary={`Регистрация: ${dateFormatter.format(new Date(user.createdAt))}`}
                />
              </ListItem>
            );
          })}
        </List>
      ) : (
        <Typography variant="body2" color="text.secondary">
          Нет данных
        </Typography>
      )}
    </CardContent>
  </Card>
);

/** Карточка «Последние платежи»: пользователь, сумма, статус, дата. */
const RecentPaymentsCard = ({ payments }: { payments: AdminStatsRecentPayment[] }) => (
  <Card sx={{ height: '100%' }}>
    <CardContent>
      <Typography variant="h6" gutterBottom>
        Последние платежи
      </Typography>
      {payments.length > 0 ? (
        <List dense disablePadding>
          {payments.map((payment) => (
            <ListItem
              key={payment.id}
              disableGutters
              secondaryAction={<PaymentStatusChip status={payment.status} />}
            >
              <ListItemText
                primary={
                  <>
                    <Link to={`/users/${payment.userId}/show`}>{payment.userPhone}</Link>
                    {' — '}
                    <Link to={`/subscriptionPayments/${payment.id}/show`}>
                      {formatKopecks(payment.amountKopecks)}
                    </Link>
                  </>
                }
                secondary={dateFormatter.format(new Date(payment.createdAt))}
              />
            </ListItem>
          ))}
        </List>
      ) : (
        <Typography variant="body2" color="text.secondary">
          Нет данных
        </Typography>
      )}
    </CardContent>
  </Card>
);

export const Dashboard = () => {
  const dataProvider = useDataProvider<AdminDataProvider>();
  const [stats, setStats] = useState<AdminStats | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [reloadToken, setReloadToken] = useState(0);

  useEffect(() => {
    let cancelled = false;
    dataProvider
      .getStats()
      .then((result: { data: unknown }) => {
        if (!cancelled) {
          setStats(result.data as AdminStats);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Не удалось загрузить статистику');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [dataProvider, reloadToken]);

  if (loading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', mt: 8 }}>
        <CircularProgress />
      </Box>
    );
  }

  if (error || !stats) {
    return (
      <Card sx={{ maxWidth: 480, mx: 'auto', mt: 8 }}>
        <CardContent>
          <Typography variant="h6" gutterBottom>
            Не удалось загрузить статистику
          </Typography>
          <Alert severity="error" sx={{ mb: 2 }}>
            {error ?? 'Неизвестная ошибка'}
          </Alert>
          <Button
            variant="contained"
            onClick={() => {
              setLoading(true);
              setError(null);
              setReloadToken((value) => value + 1);
            }}
          >
            Повторить
          </Button>
        </CardContent>
      </Card>
    );
  }

  return (
    <Box sx={{ p: 2 }}>
      <Typography variant="h5" gutterBottom>
        Дашборд
      </Typography>
      <Grid container spacing={2}>
        <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2.4 }}>
          <StatCard
            title="Пользователи"
            value={stats.usersTotal}
            subtitle={`+${countFormatter.format(stats.usersNewLast30d)} за 30 дней`}
            to="/users"
          />
        </Grid>
        <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2.4 }}>
          <StatCard title="Активные подписки" value={stats.subscriptionsActive} to={ACTIVE_SUBSCRIPTIONS_URL} />
        </Grid>
        <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2.4 }}>
          <StatCard
            title="Объекты"
            value={stats.propertiesActive}
            subtitle={`архивных: ${countFormatter.format(stats.propertiesArchived)}`}
            to="/properties"
          />
        </Grid>
        <Grid size={{ xs: 12 }}>
          <PaymentsSummaryCard stats={stats} />
        </Grid>
        <Grid size={{ xs: 12, md: 6 }}>
          <RecentUsersCard users={stats.recentUsers} />
        </Grid>
        <Grid size={{ xs: 12, md: 6 }}>
          <RecentPaymentsCard payments={stats.recentPayments} />
        </Grid>
      </Grid>
    </Box>
  );
};
