// CSS ease-out cubic-bezier(0, 0, 0.58, 1) — кривая программных переходов
// каруселей (канон замеров лент Яндекса: фиксированная длительность,
// без перелёта цели). Параметр кривой решается Ньютоном.
export function easeOut(t: number): number {
  if (t <= 0) return 0;
  if (t >= 1) return 1;
  const x2 = 0.58;
  const px = (s: number) => 3 * s * s * (1 - s) * x2 + s * s * s;
  const dx = (s: number) => 6 * s * (1 - s) * x2 + 3 * s * s;
  const py = (s: number) => 3 * s * s * (1 - s) + s * s * s;
  let curve = t;
  for (let i = 0; i < 8; i += 1) {
    const err = px(curve) - t;
    const slope = dx(curve);
    if (Math.abs(err) < 1e-6 || Math.abs(slope) < 1e-6) break;
    curve -= err / slope;
  }
  const c = Math.min(Math.max(curve, 0), 1);
  return py(c);
}
