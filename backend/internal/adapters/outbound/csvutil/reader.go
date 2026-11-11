// Package csvutil содержит общий код чтения CSV-источников.
package csvutil

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Row — строка данных CSV (без заголовка) с номером строки в файле.
type Row struct {
	Line   int
	Fields []string
}

// ReadFile читает CSV-файл, проверяет заголовок и возвращает строки данных.
//
// Ошибка уровня файла (нет файла, не тот заголовок, ошибка чтения) — это error.
// Битые строки (не то число колонок, незакрытые кавычки) пропускаются с предупреждением:
// одна плохая запись не должна останавливать весь сервис.
func ReadFile(path string, header []string) ([]Row, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1 // число колонок проверяем сами, чтобы пропускать, а не падать
	r.TrimLeadingSpace = true

	first, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read header of %s: %w", path, err)
	}
	if err := checkHeader(first, header); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	var rows []Row
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if parseErr, ok := errors.AsType[*csv.ParseError](err); ok {
				slog.Warn("csv: skip malformed row", "file", path, "line", parseErr.StartLine, "error", err)
				continue
			}
			return nil, fmt.Errorf("read %s: %w", path, err)
		}

		line, _ := r.FieldPos(0)
		if len(rec) != len(header) {
			slog.Warn("csv: skip row with wrong number of columns",
				"file", path, "line", line, "got", len(rec), "want", len(header))
			continue
		}
		for i := range rec {
			rec[i] = strings.TrimSpace(rec[i])
		}
		rows = append(rows, Row{Line: line, Fields: rec})
	}

	return rows, nil
}

func checkHeader(got, want []string) error {
	if len(got) != len(want) {
		return fmt.Errorf("unexpected header %q, want %q", got, want)
	}
	for i := range want {
		// BOM есть у файлов, сохранённых из Excel как «CSV UTF-8»
		name := strings.TrimSpace(strings.TrimPrefix(got[i], "\ufeff"))
		if !strings.EqualFold(name, want[i]) {
			return fmt.Errorf("unexpected header %q, want %q", got, want)
		}
	}
	return nil
}
