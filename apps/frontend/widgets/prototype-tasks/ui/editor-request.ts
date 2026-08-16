// ПРОТОТИП (throwaway): запрос на открытие редактора задачи.

import type { EditScope, PrototypeTask } from '../model/types';

export type EditorRequest =
    | { readonly kind: 'new'; readonly defaultDate: string }
    | {
          readonly kind: 'edit';
          readonly task: PrototypeTask;
          readonly scope: EditScope;
          readonly occurrenceDate?: string;
      };
