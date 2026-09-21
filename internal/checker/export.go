package checker

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// ImportUrls reads a txt or xlsx file exported by the crawler:
//   - .txt: one URL per line
//   - .xlsx: rows where column A == "200" and column B is the URL
func ImportUrls(path string) ([]string, error) {
	if strings.HasSuffix(strings.ToLower(path), ".xlsx") {
		return importXlsx(path)
	}
	return importTxt(path)
}

func importTxt(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line != "" {
			out = append(out, line)
		}
	}
	return out, sc.Err()
}

func importXlsx(path string) ([]string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rows, err := f.GetRows(f.GetSheetName(0))
	if err != nil {
		return nil, err
	}
	var out []string
	for i, row := range rows {
		if i == 0 {
			continue // header
		}
		if len(row) >= 2 && strings.TrimSpace(row[0]) == "200" && strings.TrimSpace(row[1]) != "" {
			out = append(out, strings.TrimSpace(row[1]))
		}
	}
	return out, nil
}

// ExportXlsx writes [URL, 收录状态] to xlsx.
func ExportXlsx(items []Item, path string) error {
	f := excelize.NewFile()
	defer f.Close()
	sheet := f.GetSheetName(0)
	f.SetCellValue(sheet, "A1", "URL")
	f.SetCellValue(sheet, "B1", "收录状态")
	for i, it := range items {
		row := i + 2
		f.SetCellValue(sheet, "A"+strconv.Itoa(row), it.URL)
		f.SetCellValue(sheet, "B"+strconv.Itoa(row), statusChinese(it.Status))
	}
	return f.SaveAs(path)
}

func statusChinese(s Status) string {
	switch s {
	case Indexed:
		return "已收录"
	case NotIndexed:
		return "未收录"
	case Error:
		return "查询失败"
	default:
		return "待查询"
	}
}
