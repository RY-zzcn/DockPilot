package agent

import "fmt"

func sscanf(str, format string, args ...any) (int, error) {
	return fmt.Sscanf(str, format, args...)
}
