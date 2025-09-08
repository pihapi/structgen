package plan

// Node — один элемент дерева: файл или каталог на определённой глубине.
type Node struct {
	Name  string // короткое имя без слэшей (один сегмент)
	Dir   bool   // это каталог?
	Depth int    // глубина относительно корня (0 — подкорень)
}

// Plan — корневое имя и список узлов по порядку.
type Plan struct {
	Root  string
	Nodes []Node
}
