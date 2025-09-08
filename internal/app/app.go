package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"structgen/internal/fsops"
	"structgen/internal/parser"
	"structgen/internal/plan"
	"structgen/internal/safety"
)

// Options — все настройки запуска утилиты.
type Options struct {
	InPath     string
	OutDir     string
	DryRun     bool
	Force      bool
	Verbose    bool
	Quiet      bool
	DirPerm    os.FileMode
	FilePerm   os.FileMode
	ExecGlobs  []string
	DBMode0600 bool
	Version    string
}

// Run — главная функция приложения: читает вход, парсит, применяет.
func Run(o Options) error {
	// 1) Открываем источник: файл или stdin.
	var r io.Reader
	if o.InPath == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(o.InPath)
		if err != nil {
			return fmt.Errorf("не удалось открыть входной файл %q: %w", o.InPath, err)
		}
		defer f.Close()
		r = f
	}

	// 2) Парсим дерево в план.
	p, err := parser.Parse(r)
	if err != nil {
		return fmt.Errorf("ошибка парсинга структуры: %w", err)
	}

	// 3) Валидируем корень: он должен быть одним сегментом без слэшей.
	if err := safety.ValidateName(p.Root); err != nil {
		return fmt.Errorf("корень проекта некорректен: %w", err)
	}

	// 4) Готовим корневой путь назначения.
	rootPath := filepath.Join(o.OutDir, p.Root)

	// 5) Применяем план к файловой системе.
	args := fsops.ApplyArgs{
		Plan:       p,
		DestRoot:   rootPath,
		DryRun:     o.DryRun,
		Force:      o.Force,
		Verbose:    o.Verbose,
		Quiet:      o.Quiet,
		DirPerm:    o.DirPerm,
		FilePerm:   o.FilePerm,
		ExecGlobs:  o.ExecGlobs,
		DBMode0600: o.DBMode0600,
	}
	if err := fsops.Apply(args); err != nil {
		return err
	}

	// 6) Готово.
	if !o.Quiet && !o.DryRun {
		fmt.Printf("Готово: %s\n", rootPath)
	}
	return nil
}

// Для удобства тестов (не обязательно)
var _ = plan.Plan{}
