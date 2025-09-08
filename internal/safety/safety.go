package safety

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidateName проверяет, что имя — один путь-сегмент без разделителей,
// не ".", не ".." и не абсолютный путь.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("пустое имя")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("недопустимое имя: %q", name)
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("имя не должно содержать разделителей пути: %q", name)
	}
	if filepath.IsAbs(name) {
		return fmt.Errorf("абсолютные пути запрещены: %q", name)
	}
	return nil
}

// SafeJoin объединяет root и parts и убеждается, что результат остаётся внутри root.
func SafeJoin(root string, parts ...string) (string, error) {
	p := filepath.Join(append([]string{root}, parts...)...)
	cleanRoot := filepath.Clean(root)
	cleanP := filepath.Clean(p)

	rel, err := filepath.Rel(cleanRoot, cleanP)
	if err != nil {
		return "", err
	}
	relSl := filepath.ToSlash(rel)
	if relSl == ".." || strings.HasPrefix(relSl, "../") {
		return "", fmt.Errorf("попытка выхода за пределы корня: %s", p)
	}
	return cleanP, nil
}
