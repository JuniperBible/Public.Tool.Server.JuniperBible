package common

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// ANSI color codes
const (
	Reset  = "\033[0m"
	Red    = "\033[0;31m"
	Green  = "\033[0;32m"
	Yellow = "\033[1;33m"
	Blue   = "\033[0;34m"
	Cyan   = "\033[0;36m"
	Bold   = "\033[1m"
)

// Banner prints the Juniper Bible ASCII art banner
func Banner(hostname, ip, osVersion, kernel string) {
	fmt.Print(Cyan)
	fmt.Println(`                                 ▄`)
	fmt.Println(`                                ▟ ▙`)
	fmt.Println(`                               ▟   ▙`)
	fmt.Println(`                              ▟     ▙`)
	fmt.Println(`                             ▟       ▙`)
	fmt.Println(`                            ▟         ▙`)
	fmt.Println(`                           ▟   ▄███▄   ▙`)
	fmt.Println(`                          ▟  ▄█▀   ▀█▄  ▙`)
	fmt.Println(`                         ▟  ██       ██  ▙`)
	fmt.Println(`                        ▟   █    ●    █   ▙`)
	fmt.Println(`                       ▟    ██       ██    ▙`)
	fmt.Println(`                      ▟      ▀█▄   ▄█▀      ▙`)
	fmt.Println(`                     ▟         ▀███▀         ▙`)
	fmt.Println(`                    ▟                         ▙`)
	fmt.Println(`                   ▟                           ▙`)
	fmt.Println(`                  ▟▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▙`)
	fmt.Println()
	fmt.Println(`               ╦╦ ╦╔╗╔╦╔═╗╔═╗╦═╗  ╔╗ ╦╔╗ ╦  ╔═╗`)
	fmt.Println(`               ║║ ║║║║║╠═╝║╣ ╠╦╝  ╠╩╗║╠╩╗║  ║╣`)
	fmt.Println(`              ╚╝╚═╝╝╚╝╩╩  ╚═╝╩╚═  ╚═╝╩╚═╝╩═╝╚═╝`)
	fmt.Print(Reset)
	fmt.Println()
	fmt.Printf("%s                Welcome to Juniper Bible Server%s\n", Bold, Reset)
	fmt.Println("                ─────────────────────────────────")
	fmt.Printf("                Hostname:  %s\n", hostname)
	fmt.Printf("                IP:        %s\n", ip)
	fmt.Printf("                OS:        NixOS %s\n", osVersion)
	fmt.Printf("                Kernel:    %s\n", kernel)
	fmt.Println()
}

// Header prints a section header
func Header(title string) {
	fmt.Println("========================================")
	fmt.Printf("%s%s%s\n", Bold, title, Reset)
	fmt.Println("========================================")
	fmt.Println()
}

// Success prints a success message
func Success(msg string) {
	fmt.Printf("%s✓ %s%s\n", Green, msg, Reset)
}

// Error prints an error message
func Error(msg string) {
	fmt.Printf("%s✗ %s%s\n", Red, msg, Reset)
}

// Warning prints a warning message
func Warning(msg string) {
	fmt.Printf("%s⚠ %s%s\n", Yellow, msg, Reset)
}

// Info prints an info message
func Info(msg string) {
	fmt.Printf("%s→ %s%s\n", Cyan, msg, Reset)
}

// Prompt asks for user input with a default value
func Prompt(question, defaultVal string) string {
	reader := bufio.NewReader(os.Stdin)
	if defaultVal != "" {
		fmt.Printf("%s [%s]: ", question, defaultVal)
	} else {
		fmt.Printf("%s: ", question)
	}
	input, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return defaultVal
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal
	}
	return input
}


// getConfirmPrompt returns the appropriate prompt string
func getConfirmPrompt(defaultYes bool) string {
	if defaultYes {
		return "[Y/n]"
	}
	return "[y/N]"
}

// parseConfirmInput parses the user input for confirmation
func parseConfirmInput(input string, defaultYes bool) bool {
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "" {
		return defaultYes
	}
	return input == "y" || input == "yes"
}

// Confirm asks for yes/no confirmation
func Confirm(question string, defaultYes bool) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s %s: ", question, getConfirmPrompt(defaultYes))
	input, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return defaultYes
	}
	return parseConfirmInput(input, defaultYes)
}

// WaitForEnter waits for the user to press Enter
func WaitForEnter(msg string) {
	reader := bufio.NewReader(os.Stdin)
	if msg == "" {
		msg = "Press Enter to continue..."
	}
	fmt.Printf("%s%s%s\n", Yellow, msg, Reset)
	_, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return
	}
}

// ClearScreen clears the terminal
func ClearScreen() {
	fmt.Print("\033[H\033[2J")
}

// Step prints a step header
func Step(num, total int, title string) {
	ClearScreen()
	fmt.Printf("%sStep %d/%d: %s%s\n\n", Bold, num, total, title, Reset)
}
