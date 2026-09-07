/**
 * Разбор подсказки адреса на строку списка (Figma 1519-94336): заголовок —
 * улица и дом, подпись — город. Реальный DaData приносит `value` полной
 * иерархической строкой («Московская обл, г Коломна, ул Суворова, д 2»),
 * `city` — отдельно; сегмент региона и города в заголовке дублировал бы
 * подпись, поэтому вырезается. Если города в значении нет (старый формат
 * «Ленина, 31»), значение остаётся целиком.
 */
export function addressSuggestionRow(
  value: string,
  city: string | undefined,
): { title: string; subtitle: string | undefined } {
  if (city === undefined || value === city) {
    return { title: value, subtitle: undefined };
  }

  const title = stripCitySegment(value, city);
  if (title.toLowerCase() === city.toLowerCase()) {
    // Значение целиком — город (в том числе с префиксом «г. »): подпись
    // дублировала бы заголовок.
    return { title, subtitle: undefined };
  }

  return { title, subtitle: city };
}

/** Вырезает из значения ведущий сегмент города («г. Москва, ») или всё до
 * конца сегмента города внутри иерархии («Московская обл, г Коломна, »);
 * для значения-города возвращает его без префикса, при отсутствии города
 * в значении — значение как есть. */
function stripCitySegment(value: string, city: string): string {
  // Город внутри иерархии: «…, г Коломна, …» (префикс «г»/«гор» с точкой
  // или без; после города — запятая). Город как подстрока другого слова
  // («Коломенская») отсечётся требованием запятой сразу после города.
  const escaped = city.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const cityInside = new RegExp(`^(?:.*?,)?\\s*(?:г|гор)\\.?\\s*${escaped}\\s*,\\s*`, 'i');
  const insideMatch = cityInside.exec(value);
  if (insideMatch !== null) {
    return value.slice(insideMatch[0].length);
  }

  // Город в начале значения («г. Москва, ул …» / «Москва, ул …»).
  const prefix = /^(?:г|гор)\.?\s+/i.exec(value);
  const rest = prefix !== null ? value.slice(prefix[0].length) : value;
  if (rest.toLowerCase().startsWith(city.toLowerCase())) {
    const afterCity = rest.slice(city.length);
    if (afterCity === '') {
      return rest;
    }
    const separator = /^[,]\s*/.exec(afterCity);
    if (separator !== null) {
      return afterCity.slice(separator[0].length);
    }
  }

  return value;
}
