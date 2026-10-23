export {
  historyFeedQueryOptions,
  historyFiltersQueryOptions,
  fetchHistoryFilters,
} from './api/queries';
export {
  useHistoryFeed,
  useHistoryFilters,
  useHistoryHasEntries,
  historyFeedScopeKey,
  lastLiveMergeAt,
} from './api/hooks';
export {
  acknowledgeFreshFeedEntryIds,
  freshFeedEntryIdsMergedSince,
} from './api/live-fresh';
export { groupHistoryByDay, type HistoryActorGroup } from './lib/feed-groups';
export {
  DEFAULT_HISTORY_FILTERS,
  historyFeedScope,
  historyParticipantSections,
  historyParticipantTitle,
  historyPeriodChipLabel,
  isDefaultHistoryFilters,
  pinnedHistoryFilters,
  toggleHistoryFilterGroup,
  toggleHistoryFilterOption,
  type HistoryFilters,
} from './lib/history-filters';
export { useHistoryFiltersState } from './lib/use-history-filters';
/** Личность страниц истории (тайтл/фолбэк «Назад» по скоупу) — чистая
 * функция без React: живой экран, Suspense-фолбэк и реестр персистентной
 * шапки (widgets/screen-layout) строят хром одним вычислением (#1605). */
export { historyScreenBackHref, historyScreenTitle } from './lib/screen-chrome';
