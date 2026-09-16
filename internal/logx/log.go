// Package logx provides colored logging helpers matching lib/common.sh.
package logx

import (
	"fmt"
	"os"
)

func Info(format string, a ...any) {
	fmt.Printf("\033[1;34m[INFO]\033[0m %s\n", fmt.Sprintf(format, a...))
}

func Warn(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "\033[1;33m[WARN]\033[0m %s\n", fmt.Sprintf(format, a...))
}

func Error(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "\033[1;31m[ERROR]\033[0m %s\n", fmt.Sprintf(format, a...))
}

// Die prints an error and exits(1), mirroring lib/common.sh's die().
func Die(format string, a ...any) {
	Error(format, a...)
	os.Exit(1)
}
