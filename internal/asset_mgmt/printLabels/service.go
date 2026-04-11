package printLabels

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

type Service struct {
}

func NewService() *Service { return &Service{} }

func (s *Service) ResolveTemplatePath(ctx context.Context, width int, barcodeType string) (string, string, error) {
	baseDir, err := os.Getwd()
	if err != nil {
		return "", "", ErrInternal("failed to resolve working directory")
	}

	err = validateTemplateRequest(width, barcodeType)
	if err != nil {
		return "", "", ErrInvalid("invalid tape width or code type")
	}

	tplDir := filepath.Join(baseDir, "internal", "asset_mgmt", "printLabels", "templates")
	filename := fmt.Sprintf("%d_%s.lw1", width, barcodeType)
	fullpath := filepath.Join(tplDir, filename)

	if _, err = os.Stat(fullpath); err != nil {
		if os.IsNotExist(err) {
			return "", "", ErrNotFound(fmt.Sprintf("template not found: %s", filename))
		}
		return "", "", ErrInternal(err.Error())
	}

	return fullpath, filename, nil
}

func validateTemplateRequest(width int, barcodeType string) error {
	switch width {
	case 9, 12, 18:
	default:
		return ErrInvalid("unsupported width")
	}

	switch barcodeType {
	case "qrcode", "code128":
	default:
		return ErrInvalid("unsupported type")
	}

	return nil
}

func (s *Service) PrintLabels(ctx context.Context, input PrintRequest) (*PrintResponse, error) {
	rows := []PrintRow{{Checked: input.Label.Checked,
		ColB: input.Label.ColB,
		ColC: input.Label.ColC,
		ColD: input.Label.ColD,
		ColE: input.Label.ColE,
	}}

	params := PrintParams{
		TemplateWidthMM:     input.Width,
		BarcodeType:         input.Type,
		UseHalfcut:          input.Config.UseHalfcut,
		ConfirmTapeWidthDlg: input.Config.ConfirmTapeWidth,
		EnablePrintLog:      input.Config.EnablePrintLog,
		PrinterName:         "",
	}

	if err := PrintLabels(rows, params); err != nil {
		if errors.Is(err, ErrTapeSizeNotMatched) {
			// テープ幅の不一致は「クライアントからの要求とサーバーの状態の競合」:409 Conflictを返す
			log.Println("[WARN]", ErrConflict(err.Error()))
			return nil, ErrConflict(err.Error())
		}
		if errors.Is(err, ErrTemplateNotFound) {
			// テンプレートが見つからない: 404 Not Found
			log.Println("[WARN]", ErrNotFound(err.Error()))
			return nil, ErrNotFound(err.Error())
		}
		if errors.Is(err, ErrNoPrintableSelected) {
			// 印刷対象が選択されていないのは「クライアントのリクエストが不正」:400 Bad Request
			log.Println("[WARN]", ErrInvalid(err.Error()))
			return nil, ErrInvalid(err.Error())
		}
		if errors.Is(err, ErrSPC10NotFound) {
			// SPC10.exeが見つからないのはサーバー内部の問題:500 Internal
			// ただし、メッセージは具体的で分かりやすいものにする
			log.Println("[ERROR]", ErrInternal(err.Error()))
			return nil, ErrInternal(err.Error())
		}

		// その他の予期せぬエラーも500 Internal
		log.Printf("[ERROR] %v\n", err)
		return nil, ErrInternal(err.Error())
	}

	return &PrintResponse{Success: true, Error: nil}, nil
}

func (s *Service) PrintLabelsBatch(ctx context.Context, input BatchPrintRequest) (*PrintResponse, error) {
	// LabelData -> PrintRow へ変換
	rows := make([]PrintRow, 0, len(input.Labels))
	for _, l := range input.Labels {
		rows = append(rows, PrintRow{
			Checked: l.Checked,
			ColB:    l.ColB,
			ColC:    l.ColC,
			ColD:    l.ColD,
			ColE:    l.ColE,
		})
	}

	params := PrintParams{
		TemplateWidthMM:     input.Width,
		BarcodeType:         input.Type,
		UseHalfcut:          input.Config.UseHalfcut,
		ConfirmTapeWidthDlg: input.Config.ConfirmTapeWidth,
		EnablePrintLog:      input.Config.EnablePrintLog,
		PrinterName:         "",
	}

	if err := PrintLabels(rows, params); err != nil {
		if errors.Is(err, ErrTapeSizeNotMatched) {
			// テープ幅の不一致は「クライアントからの要求とサーバーの状態の競合」:409 Conflictを返す
			log.Println("[WARN]", ErrConflict(err.Error()))
			return nil, ErrConflict(err.Error())
		}
		if errors.Is(err, ErrTemplateNotFound) {
			// テンプレートが見つからない: 404 Not Found
			log.Println("[WARN]", ErrNotFound(err.Error()))
			return nil, ErrNotFound(err.Error())
		}
		if errors.Is(err, ErrNoPrintableSelected) {
			// 印刷対象が選択されていないのは「クライアントのリクエストが不正」:400 Bad Request
			log.Println("[WARN]", ErrInvalid(err.Error()))
			return nil, ErrInvalid(err.Error())
		}
		if errors.Is(err, ErrSPC10NotFound) {
			// SPC10.exeが見つからないのはサーバー内部の問題:500 Internal
			// ただし、メッセージは具体的で分かりやすいものにする
			log.Println("[ERROR]", ErrInternal(err.Error()))
			return nil, ErrInternal(err.Error())
		}

		// その他の予期せぬエラーも500 Internal
		log.Printf("[ERROR] %v\n", err)
		return nil, ErrInternal(err.Error())
	}
	return &PrintResponse{Success: true, Error: nil}, nil
}
