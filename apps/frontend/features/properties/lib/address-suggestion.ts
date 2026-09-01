/**
 * Разбор подсказки адреса на строку списка (Figma 1519-94336): заголовок —
 * улица и дом, подпись — город. DaData приносит `value` полной строкой
 * («г. Москва, ул. Ленина, д. 31») и `city` отдельно; начальный сегмент
 * города в заголовке дублировал бы подпись, поэтому отрезается. Город,
 * встретившийся в середине значения (в названии улицы), не трогаем.
 */
export function addressSuggestionRow(
  value: string,
  city: string | undefined,
): { title: string; subtitle: string | undefined } {
  if (city === undefined || value === city) {
    return { title: value, subtitle: undefined };
  }

  return { title: stripLeadingCity(value, city), subtitle: city };
}

/** Отрезает ведущий сегмент города («г. Москва, » / «г Москва, »), если он
 * там есть; иначе возвращает значение как есть. */
function stripLeadingCity(value: string, city: string): string {
  const prefix = /^(?:г|гор)\.?\s+/i.exec(value);
  const rest = prefix !== null ? value.slice(prefix[0].length) : value;

  if (!rest.toLowerCase().startsWith(city.toLowerCase())) {
    return value;
  }

  const afterCity = rest.slice(city.length);
  const separator = /^[,]\s*/.exec(afterCity);

  return separator !== null ? afterCity.slice(separator[0].length) : value;
}
