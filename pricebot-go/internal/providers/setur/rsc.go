package setur

// Next.js RSC (React Flight) payload yardimcilari. Sayfa `self.__next_f.push([1,"..."])` parcalari
// icinde satir bazli bir akis tasir: `<id>:<json>` veya uzun metinler icin `<id>:T<hexUzunluk>,<metin>`.
// JSON icindeki "$<id>" string'leri bu metin parcalarina referanstir.

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	reNextPush = regexp.MustCompile(`self\.__next_f\.push\(\[1,("(?:[^"\\]|\\.)*")\]\)`)
	// reRSCHead kayit basi: `<id>:` ve istege bagli `T<hexLen>,` (metin parcasi).
	reRSCHead = regexp.MustCompile(`^([0-9a-f]+):(T([0-9a-f]+),)?`)
)

// rscPayload sayfadaki push parcalarini JS string olarak cozup birlestirir.
func rscPayload(page string) string {
	var b strings.Builder
	for _, m := range reNextPush.FindAllStringSubmatch(page, -1) {
		var s string
		if json.Unmarshal([]byte(m[1]), &s) == nil {
			b.WriteString(s)
		}
	}
	return b.String()
}

// rscTexts akisi kayit kayit okuyup `<id>:T<hexLen>,<metin>` parcalarini id -> metin haritasina cikarir.
// Uzunluk bayt cinsindendir ve metin parcasindan sonra satir sonu olmayabilir (bir sonraki kayit hemen
// baslar); bu yuzden satir bazli regex yerine sirali tarama yapilir.
func rscTexts(payload string) map[string]string {
	out := map[string]string{}
	pos := 0
	for pos < len(payload) {
		m := reRSCHead.FindStringSubmatchIndex(payload[pos:])
		if m != nil && m[4] >= 0 {
			id := payload[pos+m[2] : pos+m[3]]
			n, err := strconv.ParseInt(payload[pos+m[6]:pos+m[7]], 16, 64)
			if err == nil {
				start, end := pos+m[1], pos+m[1]+int(n)
				if end > len(payload) {
					end = len(payload)
				}
				out[id] = payload[start:end]
				pos = end
				continue
			}
		}
		// JSON kaydi (veya bozuk satir): sonraki satira gec
		nl := strings.IndexByte(payload[pos:], '\n')
		if nl < 0 {
			break
		}
		pos += nl + 1
	}
	return out
}

// rscRef "$66" gibi referanslari metin haritasindan cozer; referans degilse oldugu gibi doner,
// cozulemeyen referanslar ("$undefined") bos olur.
func rscRef(s string, texts map[string]string) string {
	if !strings.HasPrefix(s, "$") {
		return s
	}
	if t, ok := texts[s[1:]]; ok {
		return t
	}
	return ""
}

// rscExtract payload'da `key` (orn. `"initialData":`) ile baslayan ve degeri nesne/dizi olan ilk
// JSON degerini v'ye cozer ("$ref" string degerli tekrarlar atlanir).
func rscExtract(payload, key string, v any) error {
	off := 0
	for {
		idx := strings.Index(payload[off:], key)
		if idx < 0 {
			return fmt.Errorf("sayfada %s bulunamadi", key)
		}
		start := off + idx + len(key)
		rest := strings.TrimLeft(payload[start:], " ")
		if strings.HasPrefix(rest, "{") || strings.HasPrefix(rest, "[") {
			dec := json.NewDecoder(strings.NewReader(rest))
			if err := dec.Decode(v); err != nil {
				return fmt.Errorf("%s JSON cozulemedi: %w", key, err)
			}
			return nil
		}
		off = start
	}
}
