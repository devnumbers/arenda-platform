# Research: выбор Go-библиотеки для генерации xlsx

Тикет: [#51](https://github.com/devnumbers/arenda-platform/issues/51) (в рамках карты [#50](https://github.com/devnumbers/arenda-platform/issues/50)).
Дата: 2026-07-30.

## Задача

Экспорт всех данных по одному объекту недвижимости в xlsx с несколькими листами
(«Сводка», «Операции», «Аренды», «Арендаторы»). Требования:

- русские заголовки и имена листов (кириллица);
- деньги — числа в рублях с копейками через числовой формат ячейки;
- даты — настоящие даты Excel (serial + формат даты), не строки;
- потенциально тысячи строк операций — важны память и стриминг;
- чистый Go, без cgo (стек бэкенда: chi, pgx, sqlc, Go 1.26.5 — `apps/backend/go.mod`).

## Кандидаты

### 1. excelize (`github.com/xuri/excelize/v2`, репозиторий qax-os/excelize)

- **Статус:** де-факто стандарт индустрии для xlsx в Go. ~20.7k звёзд, активная
  разработка, последний релиз **v2.11.0 от 06.07.2026**. Предыдущие релизы:
  v2.10.1 (февраль 2026), v2.10.0 (октябрь 2025). Security-фиксы выходят
  оперативно (CVE-2026-54063 и др. закрыты в v2.11.0).
- **Лицензия:** BSD-3-Clause.
- **Go:** чистый Go, без cgo. v2.11.0 требует Go ≥ 1.25 — у нас Go 1.26.5, совместимо.
- **Имена:** qax-os/excelize и xuri/excelize — один проект (qax-os — организация,
  xuri — автор и канонический module path). Импорт: `github.com/xuri/excelize/v2`.
- **Несколько листов / переименование:** `NewSheet`, `SetSheetName` — полная
  поддержка Unicode/кириллицы в именах листов и значениях ячеек (проверено
  сгенерированным файлом: `<sheet name="Операции">`).
- **Стили:** `NewStyle` с `Font{Bold: true}` для заголовков; `CustomNumFmt`
  для денег (`#,##0.00" ₽"`) и дат (`DD.MM.YYYY`); `SetColWidth` /
  `AutoFitColWidth` для ширины колонок; `SetCellFloat` для чисел с копейками;
  `SetCellValue(time.Time)` пишет настоящую дату Excel.
- **Память/стриминг:** `StreamWriter` (`NewStreamWriter` + `SetRow` + `Flush`)
  пишет лист построчно без загрузки всей книги в память — для тысяч строк
  операций подходит. Есть `TmpDir` в `Options` для временных файлов.
- **Прочее:** чтение xlsx тоже есть (пригодится для тестов экспорта),
  русскоязычная документация, WASM/Python/.NET-обёртки.

### 2. tealeg/xlsx

- **Статус:** проект **мигрировал с GitHub на Codeberg** (codeberg.org/tealeg/xlsx),
  GitHub-репозиторий объявлен неподдерживаемым («No further support or
  maintenance will be provided»). Последний релиз на GitHub — v3.x (апрель 2025),
  дальше v4 на Codeberg.
- **Лицензия:** BSD-3-Clause, чистый Go.
- **Минусы:** смена хостинга = риски для go-модулей, проксирования и CI;
  сообщество и активность заметно меньше excelize; API для стилей менее
  развит. Для новой фичи — неоправданный риск.

### 3. xlsxwriter-go / goxlsxwriter (`github.com/fterrag/goxlsxwriter` и аналоги)

- Go-биндинги к C-библиотеке **libxlsxwriter** — требуют **cgo** и установленной
  C-библиотеки в системе. Последняя активность — 2022 год.
- **Отпадает** по ограничению «без cgo» (усложнение сборки и Docker-образов).

### 4. plandem/xlsx

- Последние коммиты — ~2019–2020, фактически заброшен (сам автор с 2025 года
  контрибьютит в excelize). Не рассматриваем.

## Рекомендация

**`github.com/xuri/excelize/v2`** (последняя версия на момент ресёрча — v2.11.0).

Обоснование:

1. Единственный зрелый, активно поддерживаемый pure-Go кандидат: релизы каждые
   несколько месяцев, оперативные security-фиксы, ~20.7k звёзд.
2. Покрывает все требования из коробки: мультисписочность, кириллица в именах
   листов и ячейках, кастомные числовые форматы (рубли с копейками), настоящие
   даты Excel, жирные заголовки, ширина колонок.
3. `StreamWriter` даёт построчную запись с малым потреблением памяти —
   закрывает сценарий «тысячи строк операций».
4. Совместим с Go 1.26.5 (требование — Go ≥ 1.25), без cgo — не ломает
   чистый Go-стек и кросс-компиляцию.
5. BSD-3-Clause — permissive, совместима с проектом.

## Проверка

Минимальный пример ниже собран и запущен на Go 1.26.5 с excelize v2.11.0.
Сгенерированный файл проверен: имя листа `Операции` в `workbook.xml`,
форматы `#,##0.00" ₽"` и `DD.MM.YYYY` в `styles.xml` — присутствуют.

```go
package main

import (
	"fmt"
	"time"

	"github.com/xuri/excelize/v2"
)

func main() {
	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Println(err)
		}
	}()

	sheet := "Операции"
	if _, err := f.NewSheet(sheet); err != nil {
		panic(err)
	}
	f.DeleteSheet("Sheet1")

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
	})
	if err != nil {
		panic(err)
	}
	moneyFmt := `#,##0.00" ₽"`
	moneyStyle, err := f.NewStyle(&excelize.Style{CustomNumFmt: &moneyFmt})
	if err != nil {
		panic(err)
	}
	dateFmt := "DD.MM.YYYY"
	dateStyle, err := f.NewStyle(&excelize.Style{CustomNumFmt: &dateFmt})
	if err != nil {
		panic(err)
	}

	headers := []string{"Дата", "Описание", "Сумма"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellStr(sheet, cell, h); err != nil {
			panic(err)
		}
	}
	if err := f.SetCellStyle(sheet, "A1", "C1", headerStyle); err != nil {
		panic(err)
	}

	if err := f.SetCellValue(sheet, "A2", time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)); err != nil {
		panic(err)
	}
	if err := f.SetCellStyle(sheet, "A2", "A2", dateStyle); err != nil {
		panic(err)
	}
	if err := f.SetCellStr(sheet, "B2", "Оплата аренды за июль"); err != nil {
		panic(err)
	}
	if err := f.SetCellFloat(sheet, "C2", 45000.50, 2, 64); err != nil {
		panic(err)
	}
	if err := f.SetCellStyle(sheet, "C2", "C2", moneyStyle); err != nil {
		panic(err)
	}

	if err := f.SetColWidth(sheet, "A", "A", 12); err != nil {
		panic(err)
	}
	if err := f.SetColWidth(sheet, "B", "B", 30); err != nil {
		panic(err)
	}
	if err := f.SetColWidth(sheet, "C", "C", 14); err != nil {
		panic(err)
	}

	if err := f.SaveAs("check.xlsx"); err != nil {
		panic(err)
	}
}
```

Для больших листов вместо `SetCell*` используется стриминг:

```go
sw, err := f.NewStreamWriter(sheet)
// ...
for _, op := range operations {
	row := []interface{}{
		excelize.Cell{StyleID: dateStyle, Value: op.Date},
		op.Description,
		excelize.Cell{StyleID: moneyStyle, Value: op.AmountRub},
	}
	cell, _ := excelize.CoordinatesToCellName(1, rowNum)
	if err := sw.SetRow(cell, row); err != nil {
		return err
	}
	rowNum++
}
if err := sw.Flush(); err != nil {
	return err
}
```

## Источники

- Релизы excelize: https://github.com/xuri/excelize/releases (v2.11.0, 06.07.2026)
- Документация/API: https://pkg.go.dev/github.com/xuri/excelize/v2
- Репозиторий: https://github.com/qax-os/excelize
- Статус tealeg/xlsx: https://github.com/tealeg/xlsx (миграция на Codeberg, конец поддержки на GitHub)
- goxlsxwriter (cgo): https://github.com/fterrag/goxlsxwriter
- plandem/xlsx: https://github.com/plandem/xlsx
