package cli

import (
	"fmt"
	"strings"
	"unicode"
)

// tokenize разбивает команду на аргументы.
// Кавычки нужны, чтобы можно было передавать значения с пробелами.
func tokenize(line string) ([]string, error) {
	var result []string
	var current strings.Builder
	var quote rune

	addCurrent := func() {
		if current.Len() == 0 {
			return
		}
		result = append(result, current.String())
		current.Reset()
	}

	for _, char := range line {
		if quote != 0 {
			if char == quote {
				quote = 0
				continue
			}
			current.WriteRune(char)
			continue
		}

		if char == '\'' || char == '"' {
			quote = char
			continue
		}

		if unicode.IsSpace(char) {
			addCurrent()
			continue
		}

		current.WriteRune(char)
	}

	if quote != 0 {
		return nil, fmt.Errorf("parse command: unfinished quote")
	}

	addCurrent()
	return result, nil
}
