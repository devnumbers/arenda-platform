import type { SelectOption } from '@/shared/ui/select';

export type MemberRole = 'full_access' | 'viewer';

export const memberRoleOptions: readonly SelectOption<MemberRole>[] = [
    { value: 'full_access', label: 'Полный доступ' },
    { value: 'viewer', label: 'Только просмотр' },
];

export function memberRoleLabel(role: MemberRole): string {
    switch (role) {
        case 'full_access':
            return 'Полный доступ';
        case 'viewer':
            return 'Только просмотр';
    }
}
