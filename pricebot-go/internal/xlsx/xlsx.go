// Package xlsx bagimliliksiz, tek sayfalik .xlsx uretir (Excel/Numbers/LibreOffice acar).
package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
)

// Cell string, int, float64, bool veya nil olabilir.
type Cell = any

// Write baslik + satirlari sheet'e yazar. Ilk satir kalin degildir (stil yok); sutun genislikleri icerige gore ayarlanir.
func Write(w io.Writer, sheetName string, header []string, rows [][]Cell) error {
	if sheetName == "" {
		sheetName = "Sayfa1"
	}
	zw := zip.NewWriter(w)
	add := func(name, body string) error {
		f, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = io.WriteString(f, body)
		return err
	}
	if err := add("[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`); err != nil {
		return err
	}
	if err := add("_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`); err != nil {
		return err
	}
	if err := add("xl/workbook.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="`+esc(sheetName)+`" sheetId="1" r:id="rId1"/></sheets></workbook>`); err != nil {
		return err
	}
	if err := add("xl/_rels/workbook.xml.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`); err != nil {
		return err
	}
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
	// sutun genislikleri
	widths := make([]int, len(header))
	for i, h := range header {
		widths[i] = len([]rune(h))
	}
	for _, r := range rows {
		for i := 0; i < len(r) && i < len(widths); i++ {
			if n := len([]rune(fmt.Sprint(r[i]))); n > widths[i] {
				widths[i] = n
			}
		}
	}
	if len(widths) > 0 {
		b.WriteString("<cols>")
		for i, wd := range widths {
			if wd > 60 {
				wd = 60
			}
			fmt.Fprintf(&b, `<col min="%d" max="%d" width="%d" customWidth="1"/>`, i+1, i+1, wd+3)
		}
		b.WriteString("</cols>")
	}
	b.WriteString("<sheetData>")
	writeRow := func(n int, cells []Cell) {
		fmt.Fprintf(&b, `<row r="%d">`, n)
		for i, c := range cells {
			ref := col(i) + strconv.Itoa(n)
			switch v := c.(type) {
			case nil:
			case int:
				fmt.Fprintf(&b, `<c r="%s"><v>%d</v></c>`, ref, v)
			case int64:
				fmt.Fprintf(&b, `<c r="%s"><v>%d</v></c>`, ref, v)
			case float64:
				fmt.Fprintf(&b, `<c r="%s"><v>%s</v></c>`, ref, strconv.FormatFloat(v, 'f', -1, 64))
			case bool:
				x := 0
				if v {
					x = 1
				}
				fmt.Fprintf(&b, `<c r="%s" t="b"><v>%d</v></c>`, ref, x)
			default:
				fmt.Fprintf(&b, `<c r="%s" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, ref, esc(fmt.Sprint(v)))
			}
		}
		b.WriteString("</row>")
	}
	hc := make([]Cell, len(header))
	for i, h := range header {
		hc[i] = h
	}
	writeRow(1, hc)
	for i, r := range rows {
		writeRow(i+2, r)
	}
	b.WriteString("</sheetData></worksheet>")
	if err := add("xl/worksheets/sheet1.xml", b.String()); err != nil {
		return err
	}
	return zw.Close()
}

func col(i int) string {
	s := ""
	for i >= 0 {
		s = string(rune('A'+i%26)) + s
		i = i/26 - 1
	}
	return s
}

func esc(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
