# SmartRate UI Kit

Admin paneli (`ui/`) ve musteri portali (`ui-portal/`) tarafindan paylasilan tasarim sistemi. Her iki Vite
uygulamasi `@kit` takma adiyla bu klasore baglanir (`vite.config.js` -> `resolve.alias`). Docker derlemesinde
klasor her iki node asamasina da kopyalanir.

- `src/kit.css` — tokenlar (acik/koyu tema), temel stiller, tum bilesen stilleri, mobil kurallar (<=860px).
- `src/index.js` — bilesen disa aktarimlari.
- Marka: SmartRate (logo PDF'inden vektor yeniden cizim `Logo.jsx`; palet #00D4FF / #3B82FF / #7B2CF7, notr #0B1026 ... #F1F5F9; yazi tipi Inter).

Kurallar:
- Listeler `DataTable` ile: sutunlar, satir sonu aksiyon ikonlari (`actions`), cift tiklamayla detay (`onRowDoubleClick`). Mobilde satirlar karta donusur.
- Kisa formlar `FormModal`; uzun formlar (arama, toplu ekleme) sayfa icinde `Card`.
- Onay/soru icin `useConfirm` / `usePrompt` (window.confirm/prompt kullanilmaz); geri bildirim icin `useToast`.
- Form elemanlari `Field` + `Input/Select/Textarea/Checkbox/Switch/ChipSelect/Segmented`.
- Sayfa basligi `PageHeader`; istatistik `Stat`; durum rozetleri `Badge` (ok/warn/bad/info/neutral).
