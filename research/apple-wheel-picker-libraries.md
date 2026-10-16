# Ресерч: готовые колёсные пикеры «как у Apple» (веб/React)

Задача владельца (2026-10-14): найти готовые решения с пикером-крутилкой как у iOS,
пока только найти — без выбора и внедрения. Отдельно проверить компоненты Эмиля
Ковальски. Контекст: у нас свой WheelPicker (native scroll-snap, инерция, loop 9
циклов, зона триггера #1427, реснап-механика #1429, барабан-каскад через
scroll-driven animations) — ресерч для сверки с миром.

## Веб / React — главные кандидаты

### 1. @ncdai/react-wheel-picker — самый близкий и живой
- GitHub: https://github.com/ncdai/react-wheel-picker (★ ~780, MIT)
- Доки: https://react-wheel-picker.chanhdai.com
- iOS-like колесо: плавная инерция, бесконечный loop, нативный тач + drag мышью
  + колесо мыши; unstyled (стили свои), установка через shadcn CLI
  (`npx shadcn add @ncdai/wheel-picker`), есть клавиатура и type-ahead.
- Активность: обновлялся 2026-09, бекинг Vercel OSS Program, продвижение shadcn.
- Реализация на нативном скролле (тот же подход, что наш WheelPicker) — если
  когда-нибудь будем сверять детали, смотреть его обработку реснапа/приписок.

### 2. react-mobile-picker (adcentury) — простой ветеран
- GitHub: https://github.com/adcentury/react-mobile-picker (★ ~358, MIT)
- iOS-подобный селектор, почти unstyled, мультиколонки (Picker.Column/Item),
  controlled, wheelMode для десктопа ('off'|'natural'|'normal').
- Ограничения: значения только строки; активность умеренная (2026-06).

### 3. Ionic ion-picker — канон iOS, но тянет экосистему
- https://ionicframework.com/docs/api/picker (Ionic React / web components)
- Настоящий iOS-пикер с оверлеем и кнопками; для одного пикера тащить Ionic
  core — тяжёлая зависимость, стиль Ionic не наш канон.

### 4. Framework7 Picker — классика iOS-look, чужой фреймворк
- https://framework7.io/docs/picker — выглядит как нативный пикер iOS, но
  Framework7 — самостоятельный фреймворк, в Next.js-стек не вписывается.

### 5. Mobiscroll — коммерческий эталон
- https://mobiscroll.com — самый полированный iOS-пикер на вебе, ПЛАТНАЯ
  лицензия. Упомянут для полноты, не кандидат.

## Прочее (справочно)
- 21st.dev wheel picker — тот же ncdai-компонент в каталоге shadcn-экосистемы
  (https://21st.dev/@ncdai/components/wheel-picker).
- Ванильные 3D-роллы без React (старые/нишевые): po-po/popoPicker (3D-ролл,
  бесконечный цикл), Cople/WheelPicker (копия UIPickerView).
- React Native (не наш веб-стек, для полноты): react-native-wheel-pick,
  @quidone/react-native-wheel-picker, henninghall/react-native-date-picker
  (нативные колёса на iOS).
- No-library подход (наш путь): CSS scroll-snap сам по себе достаточен —
  https://24ways.org/2019/beautiful-scrolling-experiences-without-libraries
  (наш WheelPicker ровно об этом + свои наработки реснапа и зоны триггера).

## Эмиль Ковальски — проверка (свежий срез GitHub API, 2026-10-14)
Публичные репозитории (все):
- sonner — тосты, ★ 13 040, MIT
- vaul — шиты-дровер, ★ 8 637, MIT (уже используем в шитах колёс)
- skills — AI-скиллы, ★ 46 043
Колёсного/значимого пикера выбора среди них НЕТ. Файл
`skills/prototype/PICKER.md` — не пикер выбора: это «пилюля» переключения
вариантов прототипа (служебный хром его прототипного скилла, к нашей задаче
отношения не имеет). Готового колёсного пикера Ковальски не публиковал; его
сильная сторона — motion-полировка (vaul/sonner), что мы уже используем.

## Вывод (без выбора решения — по задаче)
Мир закрыл эту задачу тем же подходом, что и мы: нативный скролл + снап +
инерция. Единственный по-настоящему живой open-source конкурент —
@ncdai/react-wheel-picker (loop, инерция, клавиатура, shadcn). Наш WheelPicker
отличается дизайн-каноном (барабан-каскад SDA), границами min/max, бесконечным
будущим годов и интеграцией с vaul-шитом; из чужих фич, которых у нас нет, —
type-ahead и свободный loop у конечных списков.
