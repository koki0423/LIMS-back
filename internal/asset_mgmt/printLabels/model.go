package printLabels

// PrintRow: 印刷1行分
type PrintRow struct {
	Checked bool   // 印刷対象フラグ
	ColB    string // CSV 1列目
	ColC    string // CSV 2列目
	ColD    string // CSV 3列目
	ColE    string // CSV 4列目
}

type PrintParams struct {
	TemplateWidthMM     int    // 期待するテンプレ幅（テンプレート名構築用）
	BarcodeType         string // バーコードのタイプ（"type"）
	UseHalfcut          bool   // 半切
	ConfirmTapeWidthDlg bool   // テープ幅確認ダイアログ
	EnablePrintLog      bool   // ログ出力
	PrinterName         string // 明示的にプリンタ指定する場合はセット（未指定なら既定）
}