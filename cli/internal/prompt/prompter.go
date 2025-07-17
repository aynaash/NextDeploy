package prompt

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type Prompter interface {
	YesNo(msg string, defaultYes bool) (bool, error)
	Select(label string, options []string) (string, error)
	ConfirmOverwrite(path string) (bool, error)
}

type cliPrompter struct {
	nonInteractive bool
}

func NewCLIPrompter(nonInteractive bool) Prompter {
	return &cliPrompter{nonInteractive}
}

func (p *cliPrompter) YesNo(msg string, defaultYes bool) (bool, error) {
	if p.nonInteractive {
		return defaultYes, nil
	}

	fmt.Printf("%s [y/n]: ", msg)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.HasPrefix(strings.ToLower(input), "y"), nil
}

func (p *cliPrompter) Select(label string, options []string) (string, error) {
	// Placeholder for selection logic
	return options[0], nil
}

func (p *cliPrompter) ConfirmOverwrite(path string) (bool, error) {
	return p.YesNo(fmt.Sprintf("Overwrite %s?", path), false)
}
