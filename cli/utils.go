package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func PromptUser(prompt string) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(prompt)

	answer, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(answer), nil
}

func ConfirmAction(prompt string) (bool, error) {
	answer, err := PromptUser(prompt + " (yes/no): ")
	if err != nil {
		return false, err
	}

	return strings.ToLower(answer) == "yes", nil
}
