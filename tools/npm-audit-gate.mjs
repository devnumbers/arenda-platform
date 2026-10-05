#!/usr/bin/env node
// Гейт npm-audit (make npm-audit) поверх `npm audit --json`: та же политика,
// что у trivy-fs с ignore-unfixed. Блокируют только gate-level (high/critical)
// уязвимости с доступным безопасным фиксом (patch/minor). Толерируются с
// предупреждением: fixAvailable === false (нет патча upstream; первый случай —
// GHSA-vfj7-8cjw-p6xm, braces <=3.0.3, ReDoS в dev-цепочке eslint-плагинов)
// и fixAvailable.isSemVerMajor === true (npm «лечит» откатом на другой
// мажор — eslint-config-next 16.3.7 → 14.2.35; это решение владельца, а не
// автоматический security-бамп). Moderate/low гейт не рассматривает — как и
// прежний `npm audit --audit-level=high`.

let raw = '';
process.stdin.on('data', (chunk) => (raw += chunk));
process.stdin.on('end', () => {
  let report;
  try {
    report = JSON.parse(raw);
  } catch (err) {
    console.error(`npm-audit-gate: unreadable npm audit output: ${err.message}`);
    process.exit(1);
  }

  const vulns = Object.values(report.vulnerabilities ?? {});
  const isGateLevel = (v) => v.severity === 'high' || v.severity === 'critical';
  // npm отдаёт false (фикса нет), true (безопасный фикс) или объект вида
  // { name, version, isSemVerMajor } — major-сдвиг требует человека.
  const hasSafeFix = (v) => v.fixAvailable === true || (typeof v.fixAvailable === 'object' && v.fixAvailable !== null && v.fixAvailable.isSemVerMajor !== true);

  const blocking = vulns.filter((v) => isGateLevel(v) && hasSafeFix(v));
  if (blocking.length > 0) {
    console.error(`npm-audit-gate: ${blocking.length} gate-level vulnerability(ies) with a safe fix available — bump, don't bypass:`);
    for (const v of blocking) {
      console.error(`  - ${v.name} ${v.range} (${v.severity}), fixAvailable: ${JSON.stringify(v.fixAvailable)}`);
    }
    process.exit(1);
  }

  const tolerated = vulns.filter((v) => isGateLevel(v) && !hasSafeFix(v));
  for (const v of tolerated) {
    const reason = v.fixAvailable === false ? 'no upstream patch' : `major shift only (${v.fixAvailable?.name}@${v.fixAvailable?.version})`;
    console.warn(`npm-audit-gate: unfixed ${v.severity} advisory tolerated (${reason}): ${v.name} ${v.range}`);
  }
  if (tolerated.length > 0) {
    console.warn(`npm-audit-gate: ${tolerated.length} advisory(ies) tolerated — revisit when upstream patches land`);
  }
});

