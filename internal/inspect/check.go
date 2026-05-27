package inspect

type Finding struct {
	File     string
	Line     int
	Field    string
	Category string // "dead-field", "append-safety"
	Level    string // "safe", "risky", "dead"
	Message  string
	Context  string // surrounding code snippet (for append safety)
}

type Checker interface {
	Name() string
	Run(dir string) ([]Finding, error)
}
