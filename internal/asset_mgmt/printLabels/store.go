// SPDX-FileCopyrightText: KING JIM sample (ported)
// SPDX-License-Identifier: MIT
//
// 備品管理ラベル印刷
// - SPC10.exe をコマンドラインで制御
//
// 主要機能：
//  1. /GT でテープ情報ファイルを出力 → 解析
//  2. 印刷対象データの CSV(cp932) 生成
//  3. オプション組み立て → 印刷実行
package printLabels

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	// "io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// ===== グローバル定数 =====

const (
	// Excelシート起点
	LineOffset   = 4
	MaxLineCount = 20

	// エラーメッセージ
	ErrorMessageNoPrintJob   = "印刷する項目が入力されていない､または印刷チェックマークが入っていないため､ラベルを印刷できません｡"
	ErrorMessageGetTapeWidth = "テープ幅が取得できません。"
	ErrorMessageTplNotFound  = "テープ幅に合ったレイアウトが存在しません。"
	ErrorMessageRunPrint     = "\"SPC10.exe\"が指定した場所に存在しません。インストール先を確認してください。"
	DefaultTemplateDummyRel  = "../bihin_12.lw1" // /GT 時に使うダミー
	DefaultSPC10PathX86      = `C:\Program Files (x86)\KING JIM\TEPRA Label Editor SPC10\SPC10.exe`
	DefaultSPC10PathX64      = `C:\Program Files\KING JIM\TEPRA Label Editor SPC10\SPC10.exe`
	TapeWidthFilename        = "TapeWidth.txt"
	PrintCSVFilename         = "data.csv"
	PrintLogFilename         = "PrintResult.txt"
	WaitTapeWidthSeconds     = 3 // /GT 後の待機
	CommandTimeout           = 60 * time.Second
)

// ===== グローバル変数・エラー定義 =====
var (
	ErrTemplateNotFound    = errors.New("template not found")
	ErrTapeSizeNotMatched  = errors.New("tape size not matched")
	ErrSPC10NotFound       = errors.New("SPC10.exe not found")
	ErrNoPrintableSelected = errors.New("no printable items selected")
)

// ===== データ構造 =====

// テープ情報（TapeWidth.txt から取得）
type TapeInfo struct {
	Width string // "12" など（mm）
	Type  string // 例: "0x00" (Standard)
}

// ===== ユーティリティ =====

// isWOW64 相当：環境変数から 64bit OS 判定
func isWOW64() bool {
	_, ok := os.LookupEnv("PROGRAMFILES(X86)")
	return ok
}

func spc10Path() (string, error) {
	candidates := []string{DefaultSPC10PathX86, DefaultSPC10PathX64}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", ErrSPC10NotFound
}

// readUTF16File UTF-16(LE/BE/BOM付想定)テキストをUTF-8で読み込み
func readUTF16File(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// BOM 付き UTF-16 として扱う（BOM 無しでも auto に乗る）
	decoder := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder()
	utf8r := transform.NewReader(bytes.NewReader(raw), decoder)
	var lines []string
	sc := bufio.NewScanner(utf8r)
	for sc.Scan() {
		lines = append(lines, strings.TrimSpace(sc.Text()))
	}
	return lines, sc.Err()
}

func writeCSVcp932(path string, rows []PrintRow) error {
	// 既定の CSV 仕様：カンマ区切り・ダブルクォート自動
	var b bytes.Buffer
	enc := japanese.ShiftJIS.NewEncoder() // Windowsの「ANSI（CP932）」相当
	w := csv.NewWriter(transform.NewWriter(&b, enc))

	for _, r := range rows {
		if !r.Checked {
			continue
		}
		record := []string{r.ColB, r.ColC, r.ColD, r.ColE}
		if err := w.Write(record); err != nil {
			return err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}
	return os.WriteFile(path, b.Bytes(), 0o644)
}

// getPrintJobCount 対象行数をカウント
func getPrintJobCount(rows []PrintRow) int {
	cnt := 0
	for _, r := range rows {
		if !r.Checked {
			continue
		}
		if r.ColB != "" || r.ColC != "" || r.ColD != "" || r.ColE != "" {
			cnt++
		}
	}
	return cnt
}

// createPrintOption SPC10-API のオプション文字列を構築
func createPrintOption(
	pathTempl string,
	pathCSV string,
	printNum int,
	halfcut bool,
	confirmTapeWidth bool,
	printLog string,
	tapeWidthFile string, // /GT の出力先（指定時のみ有効）
) string {
	parts := []string{
		pathTempl,
		pathCSV,
		fmt.Sprintf("%d", printNum),
	}

	// /GT: テープ幅出力先
	if tapeWidthFile != "" {
		parts = append(parts, "/GT "+tapeWidthFile)
	}

	// /C -f -h(半切あり) or -hn(半切なし)
	if halfcut {
		parts = append(parts, "/C -f -h")
	} else {
		parts = append(parts, "/C -f -hn")
	}

	// /TW on/off（テープ幅確認ダイアログ）
	if confirmTapeWidth {
		parts = append(parts, "/TW -on")
	} else {
		parts = append(parts, "/TW -off")
	}

	// /L ログ出力
	if printLog != "" {
		parts = append(parts, "/L "+printLog)
	}

	// Python版同様にカンマで連結
	return strings.Join(parts, ",")
}

// runSPC10 SPC10.exe の実行 (/pt あり or /p)
func runSPC10(ctx context.Context, spc10 string, option string, printerName string) error {
	var args []string
	if printerName != "" {
		args = []string{"/pt", option, printerName}
	} else {
		args = []string{"/p", option}
	}

	cmd := exec.Command(spc10, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	// デバッグ用
	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}
	return nil
}

// getTapeInfo TapeWidth.txt を解析して幅/種類を返す
func getTapeInfo(file string) (TapeInfo, error) {
	widthMap := map[string]string{
		"0x00": "0",
		"0x01": "6",
		"0x02": "9",
		"0x03": "12",
		"0x04": "18",
		"0x05": "24",
		"0x06": "36",
		"0x07": "50",
		"0x0B": "4",
		"0x21": "50",
		"0x23": "100",
		"0xFF": "",
	}

	lines, err := readUTF16File(file)
	if err != nil {
		return TapeInfo{}, err
	}
	if len(lines) < 2 {
		return TapeInfo{}, io.ErrUnexpectedEOF
	}

	// 1行目: 幅コード / 2行目: 種類コード（例: 0x00 = Standard）
	getHead := func(s string) string {
		if s == "" {
			return ""
		}
		sp := strings.SplitN(s, " ", 2)
		return sp[0]
	}
	widthCode := getHead(lines[0])
	tapeType := getHead(lines[1])

	width := widthMap[widthCode]
	return TapeInfo{Width: width, Type: tapeType}, nil
}

// fileExists 単純な存在チェック
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ===== メインの印刷フロー =====

// PrintLabels エントリポイント
func PrintLabels(data []PrintRow, p PrintParams) error {
	// 0) 絶対パス類の基準（実行時のカレント＝プロジェクトルート想定）
	baseDir, err := os.Getwd()
	if err != nil {
		return err
	}

	// 0.5) テンプレ配置ディレクトリ
	// ./LIMS-back/internal/asset_mgmt/printLabels/templates にテンプレを置いている前提
	tplDir := filepath.Join(baseDir, "internal", "asset_mgmt", "printLabels", "templates")

	// 1) SPC10.exe の場所
	spc10, err := spc10Path()
	if err != nil {
		return fmt.Errorf("%w: %s", err, ErrorMessageRunPrint)
	}

	// 2) 印刷対象があるか
	if getPrintJobCount(data) == 0 {
		return fmt.Errorf("%w: %s", ErrNoPrintableSelected, ErrorMessageNoPrintJob)
	}

	// 3) /GT でテープ幅を取得
	tapeWidthFile := filepath.Join(baseDir, TapeWidthFilename)
	printCSV := filepath.Join(baseDir, PrintCSVFilename)
	printLog := ""
	if p.EnablePrintLog {
		printLog = filepath.Join(baseDir, PrintLogFilename)
	}

	// /GT 用ダミーテンプレ（実際に存在する .lw1 を指定）
	dummyTpl := filepath.Join(tplDir, DefaultTemplateDummyRel)

	// CSV は空でも良いが、SPC10 が参照できるように用意
	if err := writeCSVcp932(printCSV, data); err != nil {
		return err
	}

	optGetWidth := createPrintOption(
		dummyTpl, printCSV, 1, p.UseHalfcut, p.ConfirmTapeWidthDlg, "", tapeWidthFile,
	)

	ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
	defer cancel()
	if err := runSPC10(ctx, spc10, optGetWidth, ""); err != nil {
		// 実行不能（PATH/権限/存在なし等）
		return fmt.Errorf("%w: %s (%v)", ErrSPC10NotFound, ErrorMessageRunPrint, err)
	}

	// SPC10 が TapeWidth.txt を出力するのを少し待つ
	time.Sleep(WaitTapeWidthSeconds)

	if !fileExists(tapeWidthFile) {
		return fmt.Errorf("%s", ErrorMessageGetTapeWidth)
	}

	ti, err := getTapeInfo(tapeWidthFile)
	if err != nil {
		return fmt.Errorf("テープ情報の読み取りに失敗: %w", err)
	}
	if ti.Width == "" || ti.Width == "0" {
		return errors.New("テープ未検出、または幅0mm")
	}
	// テープ種類のチェック（Python版と同様: 0x00=Standard のみ許容）
	if ti.Type != "0x00" {
		return fmt.Errorf("%s (Unsupported tape type: %s)", ErrorMessageTplNotFound, ti.Type)
	}

	fmt.Printf("Detected tape width: %smm\n", ti.Width)

	// 4) テンプレートの存在確認（ここも tplDir を使う）
	templateFilename := fmt.Sprintf("%d_%s.lw1", p.TemplateWidthMM, p.BarcodeType)
	templatePath := filepath.Join(tplDir, templateFilename)

	fmt.Printf("Using template file: %s\n", templatePath)

	if !fileExists(templatePath) {
		return fmt.Errorf("%w: 幅:%dmm, タイプ:%s → %s を確認してください",
			ErrTemplateNotFound, p.TemplateWidthMM, p.BarcodeType, templateFilename)
	}

	// 5) 最終 CSV 生成（Checked 行のみ）
	var filtered []PrintRow
	for _, r := range data {
		if r.Checked {
			filtered = append(filtered, r)
		}
	}
	if err := writeCSVcp932(printCSV, filtered); err != nil {
		return err
	}

	// 6) 印刷実行
	optPrint := createPrintOption(
		templatePath, printCSV, 1, p.UseHalfcut, p.ConfirmTapeWidthDlg, printLog, "",
	)

	ctx2, cancel2 := context.WithTimeout(context.Background(), CommandTimeout)
	defer cancel2()
	if err := runSPC10(ctx2, spc10, optPrint, p.PrinterName); err != nil {
		return fmt.Errorf("%s (%v)", ErrorMessageRunPrint, err)
	}

	fmt.Println("Print command sent successfully!")
	return nil
}
