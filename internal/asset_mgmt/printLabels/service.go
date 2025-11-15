package printLabels

import (
	"context"
	"errors"
	"log"
)

type Service struct {
}

func NewService() *Service { return &Service{} }

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

	return &PrintResponse{}, nil
}
