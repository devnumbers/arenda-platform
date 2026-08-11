import type {JSX} from 'react';
import {BoldKey, BoldUsers} from '@/shared/assets/icons';
import type {SharedAccessRole} from '../model/types';

export function accessRoleIcon(role: SharedAccessRole): JSX.Element {
    return role === 'viewer' ? <BoldUsers/> : <BoldKey/>;
}
