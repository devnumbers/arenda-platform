import { describe, expect, it } from 'vitest';

import {
  propertyCreateSuccessCopy,
  PROPERTY_CREATE_RENTAL_STUB_TOAST,
} from './property-create-success';

describe('propertyCreateSuccessCopy', () => {
  it('заголовок — «Объект «{название}» создан» (Figma 1425-55788)', () => {
    expect(propertyCreateSuccessCopy('Моя квартира').heading).toBe(
      'Объект «Моя квартира» создан',
    );
  });

  it('подзаголовок — дословно по макету, с обещанием аренды (override владельца 2026-09-02, #483)', () => {
    expect(propertyCreateSuccessCopy('Гараж').description).toBe(
      'Вы создали объект, теперь можете добавить аренду',
    );
  });

  it('тост-заглушка «Добавить аренду» объясняет, что раздела пока нет', () => {
    expect(PROPERTY_CREATE_RENTAL_STUB_TOAST).toBe('Раздел «Аренда» скоро появится');
  });
});
