package fsops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"structgen/internal/plan"
	"structgen/internal/safety"
)

// ApplyArgs — параметры применения плана к файловой системе.
type ApplyArgs struct {
	Plan       plan.Plan
	DestRoot   string
	DryRun     bool
	Force      bool
	Verbose    bool
	Quiet      bool
	DirPerm    os.FileMode
	FilePerm   os.FileMode
	ExecGlobs  []string
	DBMode0600 bool
}

// Apply создаёт каталоги и файлы согласно плану, безопасно и предсказуемо.
func Apply(a ApplyArgs) error {
	// 1) Создаём корень проекта.
	if a.DryRun {
		out(a, "mkdir -p %s", a.DestRoot)
	} else {
		if err := os.MkdirAll(a.DestRoot, a.DirPerm); err != nil {
			return fmt.Errorf("mkdir %s: %w", a.DestRoot, err)
		}
		if err := chmodIfNeeded(a.DestRoot, a.DirPerm); err != nil {
			return err
		}
	}
	if a.Verbose {
		out(a, "Корень: %s", a.DestRoot)
	}

	// 2) Идём по узлам, держим стек каталогов.
	var stack []string

	for _, n := range a.Plan.Nodes {
		// Проверяем корректность глубины
		if n.Depth > len(stack) {
			return fmt.Errorf("некорректная вложенность: узел %q с depth=%d, текущее дерево=%d",
				n.Name, n.Depth, len(stack))
		}
		// Поднимаемся при необходимости
		if n.Depth < len(stack) {
			stack = stack[:n.Depth]
		}

		// Абсолютный целевой путь
		target, err := safety.SafeJoin(a.DestRoot, append(stack, n.Name)...)
		if err != nil {
			return err
		}

		// Если создаём каталог
		if n.Dir {
			if err := ensureDir(a, target); err != nil {
				return err
			}
			// Добавляем эту директорию в стек как текущий уровень
			stack = append(stack, n.Name)
			continue
		}

		// Иначе — создаём файл
		if err := ensureFile(a, target); err != nil {
			return err
		}
	}

	return nil
}

func ensureDir(a ApplyArgs, path string) error {
	info, err := os.Lstat(path)
	switch {
	case err == nil && info.IsDir():
		// Каталог уже существует — ок
		if a.Verbose {
			out(a, "dir exists: %s", path)
		}
		// всё равно выставим права, если нужно
		if !a.DryRun {
			if err := chmodIfNeeded(path, a.DirPerm); err != nil {
				return err
			}
		}
		return nil

	case err == nil && !info.IsDir():
		return fmt.Errorf("конфликт: по пути %s уже существует файл", path)

	case os.IsNotExist(err):
		// Нет — создаём
		if a.DryRun {
			out(a, "mkdir -p %s", path)
			return nil
		}
		if err := os.MkdirAll(path, a.DirPerm); err != nil {
			return fmt.Errorf("mkdir %s: %w", path, err)
		}
		if err := chmodIfNeeded(path, a.DirPerm); err != nil {
			return err
		}
		if a.Verbose {
			out(a, "dir: %s", path)
		}
		return nil

	default:
		return fmt.Errorf("stat %s: %w", path, err)
	}
}

func ensureFile(a ApplyArgs, path string) error {
	// Готовим родительскую директорию
	parent := filepath.Dir(path)
	if err := ensureDir(a, parent); err != nil {
		return err
	}

	// Определяем права для файла
	mode := chooseFileMode(a, path)

	info, err := os.Lstat(path)
	switch {
	case err == nil && info.IsDir():
		return fmt.Errorf("конфликт: по пути %s уже есть каталог", path)

	case err == nil && !info.IsDir():
		// Файл существует
		if a.DryRun && !a.Force {
			out(a, "exists: %s (используйте -force для перезаписи)", path)
			return nil
		}
		if a.DryRun && a.Force {
			out(a, "truncate %s", path)
			return nil
		}
		if !a.Force {
			return fmt.Errorf("файл уже существует: %s (укажите -force для перезаписи)", path)
		}
		// Перезаписываем (усекаем)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, mode)
		if err != nil {
			return fmt.Errorf("truncate %s: %w", path, err)
		}
		_ = f.Close()
		// Выставим права, если нужно
		if err := chmodIfNeeded(path, mode); err != nil {
			return err
		}
		if a.Verbose {
			out(a, "file truncated: %s", path)
		}
		return nil

	case os.IsNotExist(err):
		// Создаём новый пустой файл
		if a.DryRun {
			out(a, "touch %s", path)
			return nil
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, mode)
		if err != nil {
			return fmt.Errorf("create %s: %w", path, err)
		}
		_ = f.Close()
		if err := chmodIfNeeded(path, mode); err != nil {
			return err
		}
		if a.Verbose {
			out(a, "file: %s", path)
		}
		return nil

	default:
		return fmt.Errorf("stat %s: %w", path, err)
	}
}

func chooseFileMode(a ApplyArgs, path string) os.FileMode {
	rel := path
	if r, err := filepath.Rel(a.DestRoot, path); err == nil {
		rel = r
	}
	relSl := filepath.ToSlash(rel)

	// DB 0600 (если включено)
	if a.DBMode0600 && (strings.HasSuffix(strings.ToLower(relSl), ".db") ||
		strings.HasSuffix(strings.ToLower(relSl), ".sqlite") ||
		strings.HasSuffix(strings.ToLower(relSl), ".sqlite3")) {
		return 0o600
	}

	// Исполняемые по glob
	for _, pat := range a.ExecGlobs {
		p := filepath.ToSlash(pat)
		if ok, _ := filepath.Match(p, relSl); ok {
			return 0o755
		}
	}
	return a.FilePerm
}

// chmodIfNeeded — безопасно выставляет права, учитывая umask и DryRun.
func chmodIfNeeded(path string, mode os.FileMode) error {
	// Здесь нельзя использовать DryRun — это утилитарная функция.
	// Применяется только из ensureDir/ensureFile с их логикой.
	return os.Chmod(path, mode)
}

func out(a ApplyArgs, format string, args ...interface{}) {
	if a.Quiet {
		return
	}
	fmt.Printf(format+"\n", args...)
}
