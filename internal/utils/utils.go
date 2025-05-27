
package utils

import (
	"fmt"
	"os"
)

func Fatal(msg string) {
	fmt.Fprintf(os.Stderr, "❌ %s\n", msg)
	os.Exit(1)
}
