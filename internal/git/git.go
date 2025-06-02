package git


import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)



func GetCommitHash() (string, error){
	cmd := exec.Command("git", "rev-parse", "--short=7", "HEAD")

	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()

	if err != nil {
		return "", fmt.Errorf("git command failed: %w", err)
	}

	return strings.TrimSpace(out.String()), nil 
}



func IsDirty() bool{
	cmd := exec.Command("git", "status", "--porcelain")

	var out bytes.Buffer

	cmd.Stdout = &out
	_ = cmd.Run()
	return out.Len() > 0
}
