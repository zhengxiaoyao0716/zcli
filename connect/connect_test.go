package connect

import (
	"fmt"
	"testing"
)

func TestIota(*testing.T) {
	fmt.Printf("%05o %05o %05o\n", ModeRoot, ModeUser, ModeGuest)
	fmt.Printf("%05o %05o %05o\n", ModeRx, ModeRw, ModeRr)
	fmt.Printf("%05o %05o %05o\n", ModeUx, ModeUw, ModeUr)
	fmt.Printf("%05o %05o %05o\n", ModeGx, ModeGw, ModeGr)
	fmt.Printf("%05o\n", ModeBan)
}
