type TenantName = {
  readonly name: string;
  readonly surname: string | null;
  readonly patronymic: string | null;
};

export function getTenantContactFullName(tenant: TenantName): string {
  return [tenant.surname, tenant.name, tenant.patronymic]
    .filter(Boolean)
    .join(' ');
}
