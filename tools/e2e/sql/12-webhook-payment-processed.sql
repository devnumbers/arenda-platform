SELECT EXISTS (
  SELECT 1
  FROM subscription_payments sp
  JOIN subscriptions s ON s.id = sp.subscription_id
  JOIN users u ON u.id = s.user_id
  WHERE u.phone = :phone
    AND sp.id = :payment_id
    AND sp.status = 'succeeded'
) AS ok;
