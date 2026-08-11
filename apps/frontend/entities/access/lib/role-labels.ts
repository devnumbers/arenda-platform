import type {SharedAccessRole} from '../model/types';

export const ACCESS_ROLE_LABELS: Readonly<Record<SharedAccessRole, string>> = {
    viewer: 'Просмотр',
    full_access: 'Полный доступ',
};
