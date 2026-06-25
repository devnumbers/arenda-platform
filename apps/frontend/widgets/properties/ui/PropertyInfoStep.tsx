import type { JSX } from 'react';

export type PropertyInfoStepProps = {
  onNext: () => void;
  onBack?: () => void;
};

export function PropertyInfoStep({}: PropertyInfoStepProps): JSX.Element {
  return <div>Step 3</div>;
}
