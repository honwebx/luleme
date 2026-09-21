package submitter

import (
	"strconv"

	"github.com/xuri/excelize/v2"
)

// ExportXlsx writes [URL, 提交状态, 失败原因] to xlsx.
func ExportXlsx(items []Item, path string) error {
	f := excelize.NewFile()
	defer f.Close()
	sheet := f.GetSheetName(0)
	f.SetCellValue(sheet, "A1", "URL")
	f.SetCellValue(sheet, "B1", "提交状态")
	f.SetCellValue(sheet, "C1", "失败原因")
	for i, it := range items {
		row := strconv.Itoa(i + 2)
		f.SetCellValue(sheet, "A"+row, it.URL)
		f.SetCellValue(sheet, "B"+row, statusChinese(it.Status))
		f.SetCellValue(sheet, "C"+row, it.Reason)
	}
	return f.SaveAs(path)
}

func statusChinese(s Status) string {
	switch s {
	case Success:
		return "成功"
	case Failed:
		return "失败"
	default:
		return "待提交"
	}
}
