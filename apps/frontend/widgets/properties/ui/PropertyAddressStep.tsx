import type { JSX } from 'react';

export type PropertyAddressStepProps = {
  onNext: () => void;
  onBack?: () => void;
};

export function PropertyAddressStep({}: PropertyAddressStepProps): JSX.Element {
  return <div>Step 2</div>;
}
