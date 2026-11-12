package xlsx

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestWrite(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, "Oteller", []string{"Kimlik", "Ad", "Yıldız"}, [][]Cell{{"ext_1", "A & B <Otel>", 5}, {"ext_2", "Ç", nil}}); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var sheet string
	for _, f := range zr.File {
		if f.Name == "xl/worksheets/sheet1.xml" {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			sheet = string(b)
		}
	}
	if !strings.Contains(sheet, "A &amp; B &lt;Otel&gt;") || !strings.Contains(sheet, `<c r="C2"><v>5</v></c>`) || !strings.Contains(sheet, `r="B3"`) {
		t.Fatalf("beklenmeyen sheet: %s", sheet)
	}
	if col(26) != "AA" || col(0) != "A" {
		t.Fatal("col")
	}
}
