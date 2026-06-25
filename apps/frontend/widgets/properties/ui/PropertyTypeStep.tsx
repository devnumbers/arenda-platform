import type { JSX } from 'react';

export type PropertyTypeStepProps = {
  onNext: () => void;
  onBack?: () => void;
};

export function PropertyTypeStep({}: PropertyTypeStepProps): JSX.Element {
  return <div>Step 1</div>;
}
