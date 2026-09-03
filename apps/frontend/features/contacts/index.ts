export {
  useContacts,
  useCreateContact,
  useContact,
  useUpdateContact,
  useDeleteContact,
} from './api/hooks';
export {
  buildContactCreateCommand,
  buildContactUpdateCommand,
  contactFormErrors,
  contactFormReady,
  contactServerFieldErrors,
  isContactFormField,
} from './lib/contact-form';
export type { ContactFormFields } from './lib/contact-form';
