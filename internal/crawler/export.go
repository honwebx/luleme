package crawler

import (
	"strconv"

	"github.com/xuri/excelize/v2"
)

// ExportXlsx writes crawl items to a two-column xlsx (status, url).
func ExportXlsx(items []Item, path string) error {
	f := excelize.NewFile()
	defer f.Close()
	sheet := f.GetSheetName(0)
	f.SetCellValue(sheet, "A1", "状态码")
	f.SetCellValue(sheet, "B1", "URL")
	for i, it := range items {
		row := i + 2
		f.SetCellValue(sheet, "A"+strconv.Itoa(row), it.Status)
		f.SetCellValue(sheet, "B"+strconv.Itoa(row), it.URL)
	}
	return f.SaveAs(path)
}
