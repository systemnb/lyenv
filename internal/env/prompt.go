package env

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

func PromptLine(prompt string) (string, error) {
	fmt.Println(prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		// In case of EOF without newline, still accept the buffer
		if errors.Is(err, os.ErrClosed) {
			return strings.TrimSpace(line), nil
		}
		if line != "" {
			return strings.TrimSpace(line), nil
		}
		return "", err
	}
	return strings.TrimSpace(line), nil
}
