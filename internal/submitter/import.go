package submitter

import (
	"bufio"
	"os"
	"strings"

	"github.com/xuri/excelize/v2"
)

// MaxImportURLs caps a single file import to bound memory and avoid
// hammering IndexNow with an accidental million-row file.
const MaxImportURLs = 100000

// isURLLine reports whether a trimmed line looks like an http(s) URL.
func isURLLine(s string) bool {
	lower := strings.ToLower(s)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}
// ImportUrls reads a txt or xlsx file:
//   - .txt: lines starting with http(s):// (case-insensitive)
//   - .xlsx: finds a column whose header is "url" or "网址", reads rows whose
//     value starts with http(s)://. Capped at MaxImportURLs.
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
		if isURLLine(line) {
			out = append(out, line)
			if len(out) >= MaxImportURLs {
				break
			}
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
	urlCol := 0
	if len(rows) > 0 {
		for i, h := range rows[0] {
			hh := strings.ToLower(strings.TrimSpace(h))
			if hh == "url" || hh == "网址" {
				urlCol = i
				break
			}
		}
	}
	var out []string
	for i, row := range rows {
		if i == 0 {
			continue
		}
		if urlCol >= len(row) {
			continue
		}
		v := strings.TrimSpace(row[urlCol])
		if isURLLine(v) {
			out = append(out, v)
			if len(out) >= MaxImportURLs {
				break
			}
		}
	}
	return out, nil
}
