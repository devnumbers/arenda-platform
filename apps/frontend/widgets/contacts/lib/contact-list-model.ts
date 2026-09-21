import { contactFullName, type ContactSortOrder } from '@/entities/contact';
import type { Contact } from '@/entities/contact';
import { parseEnumParam } from '@/shared/lib/parse-enum-param';

/**
 * Модель строки списка (#508, макет 1527:74139): заголовок — имя, подзаголовок
 * — роль (без роли строки-подзаголовка нет). Телефон в строке не показывается.
 * Сортировка и алфавитные группы книги — общая логика в entities/contact
 * (нужна и шагу «Контакт арендатора» визарда аренды #530).
 */
export function contactRowModel(contact: Contact): {
  title: string;
  subtitle: string | undefined;
} {
  return {
    title: contactFullName(contact),
    subtitle: contact.role.length > 0 ? contact.role : undefined,
  };
}

/** Разбор ?order= экрана «Контакты объекта» (конвенция страницы
 * «Объекты»): неизвестное и отсутствующее значения — дефолт «А→Я».
 * Направление живёт в адресе — переживает перезагрузку (#785). */
export function parseContactListOrderParams(
  order: string | string[] | undefined,
): ContactSortOrder {
  return parseEnumParam(order, ['asc', 'desc'], 'asc');
}
