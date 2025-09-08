package parser

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"structgen/internal/plan"
	"structgen/internal/safety"
)

// Parse читает tree-подобный текст и возвращает план.
// Поддерживает псевдографику (├──/└──) и ASCII (|--/` + "`--" + `).
// Каталог определяется либо по суффиксу "/", либо по дочерним элементам (второй проход).
func Parse(r io.Reader) (plan.Plan, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1024), 1024*1024)

	var root string
	var nodes []plan.Node
	lineNum := 0

	for sc.Scan() {
		lineNum++
		raw := strings.TrimRight(sc.Text(), "\r\n")
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

		// Первая непустая строка — корень
		if root == "" {
			rootName := strings.TrimSuffix(line, "/")
			if err := safety.ValidateName(rootName); err != nil {
				return plan.Plan{}, fmt.Errorf("строка %d: некорректное имя корня: %w", lineNum, err)
			}
			root = rootName
			continue
		}

		// Игнорируем возможную итоговую строку tree "N directories, M files"
		if isTreeSummary(line) {
			continue
		}

		depth, name, ok := parseTreeLine(raw)
		if !ok {
			return plan.Plan{}, fmt.Errorf("строка %d: не похоже на строку tree: %q", lineNum, raw)
		}

		isDir := strings.HasSuffix(name, "/")
		name = strings.TrimSuffix(name, "/")

		if err := safety.ValidateName(name); err != nil {
			return plan.Plan{}, fmt.Errorf("строка %d: %w", lineNum, err)
		}

		nodes = append(nodes, plan.Node{
			Name:  name,
			Dir:   isDir,
			Depth: depth,
		})
	}
	if err := sc.Err(); err != nil {
		return plan.Plan{}, err
	}
	if root == "" {
		return plan.Plan{}, fmt.Errorf("не найден корень проекта")
	}

	// Второй проход: если у узла следующая строка глубже — это каталог.
	for i := range nodes {
		if !nodes[i].Dir {
			if i+1 < len(nodes) && nodes[i+1].Depth > nodes[i].Depth {
				nodes[i].Dir = true
			}
		}
	}

	return plan.Plan{Root: root, Nodes: nodes}, nil
}

// parseTreeLine пытается разобрать строку формата tree.
// Возвращает depth (количество уровней), имя узла и признак успеха.
func parseTreeLine(line string) (int, string, bool) {
	// Ищем любой из допустимых маркеров ветвления.
	markers := []string{"├── ", "└── ", "|-- ", "`-- ", "+-- "}
	idx := -1
	used := ""
	for _, m := range markers {
		if i := strings.Index(line, m); i != -1 && (idx == -1 || i < idx) {
			idx = i
			used = m
		}
	}
	if idx == -1 {
		// Пробуем без пробела после маркера (на всякий случай)
		markers = []string{"├──", "└──", "|--", "`--", "+--"}
		for _, m := range markers {
			if i := strings.Index(line, m); i != -1 && (idx == -1 || i < idx) {
				idx = i
				used = m
			}
		}
	}
	if idx == -1 {
		return 0, "", false
	}

	prefix := line[:idx]
	depth := countDepth(prefix)
	name := strings.TrimSpace(line[idx+len(used):])
	return depth, name, true
}

// countDepth считает глубину по префиксу.
// Заменяем все псевдографические символы и '|' на пробелы и считаем группы по 4 пробела.
func countDepth(prefix string) int {
	s := prefix
	repl := []string{"│", "└", "├", "─", "|"}
	for _, r := range repl {
		s = strings.ReplaceAll(s, r, " ")
	}
	spaces := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' {
			spaces++
		}
	}
	return spaces / 4
}

// Очень простая эвристика: игнорируем строку-резюме tree.
func isTreeSummary(line string) bool {
	s := strings.TrimSpace(strings.ToLower(line))
	// Примитивная проверка, но практичная
	return (strings.Contains(s, "directories") || strings.Contains(s, "directory")) &&
		(strings.Contains(s, "files") || strings.Contains(s, "file"))
}
