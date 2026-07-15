export type {
  ErrorScenarioFn,
  NotificationAction,
  NotificationKey,
  ScenarioFn,
  ScenarioOptions,
  PromiseScenarioFn,
  ToastOptions,
} from './types';
export type { Scenarios } from './scenarios';

import { notify as baseNotify } from './base';
import { scenarios } from './scenarios';
import type { NotifyBase } from './base';
import type { Scenarios } from './scenarios';

export type Notify = NotifyBase & {
  readonly scenarios: Scenarios;
};

export const notify: Notify = Object.assign({}, baseNotify, { scenarios });
