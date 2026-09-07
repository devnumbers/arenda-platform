import type {JSX} from 'react';
import {BoldKey, Team} from '@/shared/assets/icons';
import type {SharedAccessRole} from '../model/types';

export function accessRoleIcon(role: SharedAccessRole): JSX.Element {
    return role === 'viewer' ? <Team/> : <BoldKey/>;
}
