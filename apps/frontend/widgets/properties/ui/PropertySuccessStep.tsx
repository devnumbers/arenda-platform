import type { JSX } from 'react';

export type PropertySuccessStepProps = {
  onNext: () => void;
  onBack?: () => void;
};

export function PropertySuccessStep({}: PropertySuccessStepProps): JSX.Element {
  return <div>Step 4</div>;
}
