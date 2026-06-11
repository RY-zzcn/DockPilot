package version

import (
	"strconv"
	"strings"
)

var (
	Version   = "0.2.15"
	Commit    = "dev"
	BuildDate = "unknown"
)

type BuildInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
}

func Current() BuildInfo {
	return BuildInfo{
		Version:   Version,
		Commit:    Commit,
		BuildDate: BuildDate,
	}
}

func Clean(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "v")
	return value
}

func EnsureVPrefix(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "latest" || strings.HasPrefix(value, "v") {
		return value
	}
	return "v" + value
}

func Compare(a, b string) int {
	a = Clean(a)
	b = Clean(b)
	if a == b {
		return 0
	}
	if a == "" {
		return -1
	}
	if b == "" {
		return 1
	}
	aHasDigit := hasDigit(a)
	bHasDigit := hasDigit(b)
	if !aHasDigit && bHasDigit {
		return -1
	}
	if aHasDigit && !bHasDigit {
		return 1
	}
	left, leftPre := semverParts(a)
	right, rightPre := semverParts(b)
	for i := 0; i < len(left) || i < len(right); i++ {
		lv, rv := 0, 0
		if i < len(left) {
			lv = left[i]
		}
		if i < len(right) {
			rv = right[i]
		}
		if lv < rv {
			return -1
		}
		if lv > rv {
			return 1
		}
	}
	if leftPre == "" && rightPre != "" {
		return 1
	}
	if leftPre != "" && rightPre == "" {
		return -1
	}
	return strings.Compare(leftPre, rightPre)
}

func IsOutdated(current, latest string) bool {
	return Clean(latest) != "" && Compare(current, latest) < 0
}

func hasDigit(value string) bool {
	for _, r := range value {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

func semverParts(value string) ([]int, string) {
	value = Clean(value)
	value = strings.SplitN(value, "+", 2)[0]
	pre := ""
	if before, after, ok := strings.Cut(value, "-"); ok {
		value = before
		pre = after
	}
	fields := strings.Split(value, ".")
	parts := make([]int, 0, len(fields))
	for _, field := range fields {
		if field == "" {
			parts = append(parts, 0)
			continue
		}
		number := 0
		for _, r := range field {
			if r < '0' || r > '9' {
				break
			}
			number = number*10 + int(r-'0')
		}
		if parsed, err := strconv.Atoi(field); err == nil {
			number = parsed
		}
		parts = append(parts, number)
	}
	return parts, pre
}
