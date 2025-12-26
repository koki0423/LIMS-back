package printLabels

// ===== Requests =====
// PrintRequest: /assets/print
type PrintRequest struct {
	Config PrintConfig `json:"config" binding:"required"`
	Label  LabelData   `json:"label"  binding:"required"`
	Width  int         `json:"width"  binding:"required"`
	Type   string      `json:"type"   binding:"required"`
}

// BatchPrintRequest: /print/batch
type BatchPrintRequest struct {
	Config PrintConfig `json:"config" binding:"required"`
	Labels []LabelData `json:"labels" binding:"required"`
	Width  int         `json:"width"  binding:"required"`
	Type   string      `json:"type"   binding:"required"`
}

type PrintConfig struct {
	UseHalfcut       bool `json:"use_halfcut"`
	ConfirmTapeWidth bool `json:"confirm_tape_width"`
	EnablePrintLog   bool `json:"enable_print_log"`
}

type LabelData struct {
	Checked bool   `json:"checked" binding:"required"`
	ColB    string `json:"col_b"   binding:"required"`
	ColC    string `json:"col_c"   binding:"required"`
	ColD    string `json:"col_d"   binding:"required"`
	ColE    string `json:"col_e"   binding:"required"`
}

// ===== Responses =====
type PrintResponse struct {
	Success bool      `json:"success"`
	Error   *APIError `json:"error,omitempty"`
}

// リクエスト例
/*
	{
		"config": {
			"use_halfcut": true,
			"confirm_tape_width": true,
			"enable_print_log":false
		},
		"label": {
			"checked": true,
			"col_b": "テストアセット",
			"col_c": "個人",
			"col_d": "OFS-12340506-99999",
			"col_e": "required"
		},
		"width": 12,
		"type": "qrcode"
	}
*/
