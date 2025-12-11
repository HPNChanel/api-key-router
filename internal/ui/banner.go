// Package ui provides cyberpunk-styled console output for the HPN Router.
package ui

import (
	"fmt"

	"github.com/fatih/color"
)

// ══════════════════════════════════════════════════════════════════════════════
// ASCII ART BANNER - Cyberpunk Theme
// ══════════════════════════════════════════════════════════════════════════════

// PrintBanner displays the ASCII art startup banner with cyberpunk styling.
func PrintBanner() {
	// Clear some space
	fmt.Println()

	// Define colors for gradient effect
	cyan := color.New(color.FgCyan, color.Bold)
	magenta := color.New(color.FgMagenta, color.Bold)
	hiCyan := color.New(color.FgHiCyan)
	hiMagenta := color.New(color.FgHiMagenta)
	yellow := color.New(color.FgYellow, color.Bold)
	white := color.New(color.FgWhite)
	dim := color.New(color.FgHiBlack)

	// Top border
	cyan.Println("╔══════════════════════════════════════════════════════════════════════╗")

	// HPN ROUTER ASCII Art with gradient
	cyan.Print("║  ")
	hiCyan.Print("██╗  ██╗")
	white.Print("██████╗ ")
	hiMagenta.Print("███╗   ██╗")
	dim.Print("    ")
	magenta.Print("██████╗  ██████╗ ██╗   ██╗████████╗███████╗██████╗ ")
	cyan.Println(" ║")

	cyan.Print("║  ")
	hiCyan.Print("██║  ██║")
	white.Print("██╔══██╗")
	hiMagenta.Print("████╗  ██║")
	dim.Print("    ")
	magenta.Print("██╔══██╗██╔═══██╗██║   ██║╚══██╔══╝██╔════╝██╔══██╗")
	cyan.Println(" ║")

	cyan.Print("║  ")
	hiCyan.Print("███████║")
	white.Print("██████╔╝")
	hiMagenta.Print("██╔██╗ ██║")
	dim.Print("    ")
	magenta.Print("██████╔╝██║   ██║██║   ██║   ██║   █████╗  ██████╔╝")
	cyan.Println(" ║")

	cyan.Print("║  ")
	hiCyan.Print("██╔══██║")
	white.Print("██╔═══╝ ")
	hiMagenta.Print("██║╚██╗██║")
	dim.Print("    ")
	magenta.Print("██╔══██╗██║   ██║██║   ██║   ██║   ██╔══╝  ██╔══██╗")
	cyan.Println(" ║")

	cyan.Print("║  ")
	hiCyan.Print("██║  ██║")
	white.Print("██║     ")
	hiMagenta.Print("██║ ╚████║")
	dim.Print("    ")
	magenta.Print("██║  ██║╚██████╔╝╚██████╔╝   ██║   ███████╗██║  ██║")
	cyan.Println(" ║")

	cyan.Print("║  ")
	hiCyan.Print("╚═╝  ╚═╝")
	white.Print("╚═╝     ")
	hiMagenta.Print("╚═╝  ╚═══╝")
	dim.Print("    ")
	magenta.Print("╚═╝  ╚═╝ ╚═════╝  ╚═════╝    ╚═╝   ╚══════╝╚═╝  ╚═╝")
	cyan.Println(" ║")

	// Middle separator
	cyan.Println("╠══════════════════════════════════════════════════════════════════════╣")

	// Info line
	cyan.Print("║  ")
	yellow.Print("🔥 API KEY ROUTER")
	dim.Print("  │  ")
	hiMagenta.Print("IMMORTAL MODE ENABLED")
	dim.Print("  │  ")
	white.Print("v1.0.0")
	dim.Print("                       ")
	cyan.Println("║")

	// Bottom border
	cyan.Println("╚══════════════════════════════════════════════════════════════════════╝")

	fmt.Println()
}

// PrintMiniBanner displays a smaller, simpler banner for constrained terminals.
func PrintMiniBanner() {
	cyan := color.New(color.FgCyan, color.Bold)
	magenta := color.New(color.FgMagenta, color.Bold)
	yellow := color.New(color.FgYellow)

	fmt.Println()
	cyan.Print("╔══════════════════════════════════════╗")
	fmt.Println()
	cyan.Print("║  ")
	magenta.Print("HPN ROUTER")
	yellow.Print(" 🔥 ")
	cyan.Print("IMMORTAL MODE  ")
	cyan.Print("║")
	fmt.Println()
	cyan.Print("╚══════════════════════════════════════╝")
	fmt.Println()
	fmt.Println()
}
