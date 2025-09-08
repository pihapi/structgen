package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"structgen/internal/app"
)

// Версию можно переопределить через -ldflags "-X main.version=1.0.0"
var version = "dev"

func main() {
	// Флаги. Делаем их очевидными и простыми.
	in := flag.String("in", "struct", "Путь к входному файлу со структурой ('-' для stdin)")
	out := flag.String("out", ".", "Каталог, куда создавать проект (родитель корня)")
	dry := flag.Bool("dry", false, "Dry-run: только показать, что будет создано")
	force := flag.Bool("force", false, "Перезаписывать существующие файлы (если они уже есть)")
	verbose := flag.Bool("v", false, "Подробный вывод")
	quiet := flag.Bool("q", false, "Тихий режим (подавить обычные сообщения)")

	// Права по умолчанию: каталоги 0755, файлы 0644
	dpermStr := flag.String("dperm", "0755", "Права для каталогов (восьмерично, например 0755)")
	fpermStr := flag.String("fperm", "0644", "Права для файлов (восьмерично, например 0644)")

	// Выполняемые файлы: по шаблонам. Примеры: \"*.sh,bin/*\"
	execGlob := flag.String("exec-glob", "", "Список glob-шаблонов для исполняемых файлов (через запятую)")

	// Для .db/.sqlite иногда лучше 0600 (опционально)
	db0600 := flag.Bool("db-0600", false, "Ставить 0600 на файлы *.db/*.sqlite/*.sqlite3")

	// Справка и версия
	help := flag.Bool("help", false, "Показать справку и выйти")
	helpShort := flag.Bool("h", false, "Показать справку и выйти (синоним -help)")
	showVersion := flag.Bool("version", false, "Показать версию и выйти")

	flag.Usage = func() {
		name := filepath.Base(os.Args[0])
		fmt.Fprintf(os.Stdout, `
%s — создаёт проектную структуру из файла с tree-подобным деревом.

Использование:
  %s -in struct [-out DIR] [-dry] [-force] [-v|-q] [-dperm 0755] [-fperm 0644] [-exec-glob "*.sh,bin/*"] [-db-0600]

Флаги:
`, name, name)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stdout, `
Формат входного файла:
  Первая строка — корень (пример: project-name/).
  Далее строки с отступами и ветками вида ├──/└── или |--/`+"`--"+` (оба формата поддерживаются).
  Каталоги можно указывать с / в конце. Если / нет, но у узла есть дочерние — он будет воспринят как каталог.

Примеры:
  %[1]s -in struct -out .
  cat struct | %[1]s -in - -out ./dst -v
  %[1]s -in struct -dry -v
`, name)
	}

	// Если без аргументов — просто показать помощь
	if len(os.Args) == 1 {
		flag.Usage()
		return
	}

	flag.Parse()

	if *help || *helpShort {
		flag.Usage()
		return
	}

	if *showVersion {
		fmt.Println(version)
		return
	}

	// Разбор прав доступа
	dperm, err := parsePerm(*dpermStr, 0o755)
	if err != nil {
		fail(fmt.Errorf("неверные права -dperm: %w", err))
	}
	fperm, err := parsePerm(*fpermStr, 0o644)
	if err != nil {
		fail(fmt.Errorf("неверные права -fperm: %w", err))
	}

	// Разбор шаблонов исполняемых файлов
	execGlobs := splitGlobs(*execGlob)

	opts := app.Options{
		InPath:     *in,
		OutDir:     *out,
		DryRun:     *dry,
		Force:      *force,
		Verbose:    *verbose,
		Quiet:      *quiet,
		DirPerm:    dperm,
		FilePerm:   fperm,
		ExecGlobs:  execGlobs,
		DBMode0600: *db0600,
		Version:    version,
	}

	if err := app.Run(opts); err != nil {
		fail(err)
	}
}

func parsePerm(s string, def os.FileMode) (os.FileMode, error) {
	ss := strings.TrimSpace(s)
	if ss == "" {
		return def, nil
	}
	// base=0 понимает 0755/755/0o755
	u, err := strconv.ParseUint(ss, 0, 32)
	if err != nil {
		return 0, err
	}
	return os.FileMode(u), nil
}

func splitGlobs(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "ошибка: %v\n", err)
	os.Exit(1)
}
