import type {JSX} from 'react';
import {ACCESS_ROLE_LABELS} from '../lib/role-labels';
import {accessRoleIcon} from '../lib/role-icons';
import type {SharedAccessRole} from '../model/types';
import styles from './AccessRoleBadge.module.css';

export type AccessRoleBadgeProps = {
    readonly role: SharedAccessRole;
};

export function AccessRoleBadge({role}: AccessRoleBadgeProps): JSX.Element {
    return (
        <span
            className={styles.root}
            title={ACCESS_ROLE_LABELS[role]}
            aria-label={`Роль: ${ACCESS_ROLE_LABELS[role]}`}
        >
            {accessRoleIcon(role)}
        </span>
    );
}
