/**
 * Множество платежей с накопленной просрочкой (красная точка на иконке
 * правила — семантика payments/CONTEXT.md): проекция просроченных
 * операций над их paymentId. Единственное место правила — деталь объекта
 * (#589) и плашки «Платежей объекта»/каталога (1323:61133) бейджа́т по
 * одному множеству из usePropertyOverdueOperations.
 */
export function overduePaymentIdsOf(
  operations: ReadonlyArray<{ readonly paymentId: string | null }>,
): ReadonlySet<string> {
  return new Set(
    operations.flatMap((operation) =>
      operation.paymentId !== null ? [operation.paymentId] : [],
    ),
  );
}
