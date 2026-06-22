SELECT EXISTS (
  SELECT 1
  FROM subscription_payments sp
  JOIN user_subscriptions s ON s.id = sp.subscription_id
  JOIN users u ON u.id = sp.user_id
  WHERE u.phone = :'phone'
    AND sp.id = :'payment_id'
    AND sp.status = 'succeeded'
) AS ok;
