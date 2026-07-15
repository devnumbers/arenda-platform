export type NotificationAction = Readonly<{
  label: string;
  href?: string;
  onPress?: () => void;
}>;

export type ScenarioOptions = Readonly<{
  action?: NotificationAction;
  description?: string;
  duration?: number;
}>;

export type ToastOptions = ScenarioOptions;

export type NotificationKey = string;

export type ScenarioFn = (options?: ScenarioOptions) => NotificationKey;

export type ErrorScenarioFn = (
  error: unknown,
  options?: ScenarioOptions,
) => NotificationKey;

export type PromiseScenarioFn = <T>(
  promise: Promise<T>,
  options?: ScenarioOptions,
) => NotificationKey;
