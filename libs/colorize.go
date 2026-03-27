package libs

import "fmt"

type Color string

const (
	Red    Color = "\033[31m"
	Green  Color = "\033[32m"
	Yellow Color = "\033[33m"
	Blue   Color = "\033[34m"
	Reset  Color = "\033[0m"
)

func Colorize(color Color, text string) {
	fmt.Printf("%s%s%s", color, text, Reset)
}
