SELECT EXISTS (
  SELECT 1
  FROM subscription_payments sp
  JOIN user_subscriptions s ON s.id = sp.subscription_id
  WHERE sp.id = :'payment_id'
    AND sp.status = 'succeeded'
) AS ok;
