// Package ui provides cyberpunk-styled console output for the HPN Router.
// It creates a visually impressive terminal experience with colorized logs,
// status badges, and ASCII art.
package ui

import (
	"fmt"
	"time"

	"github.com/fatih/color"
)

// ══════════════════════════════════════════════════════════════════════════════
// COLOR DEFINITIONS - Cyberpunk Theme
// ══════════════════════════════════════════════════════════════════════════════

var (
	// Badge colors
	successBadge   = color.New(color.BgGreen, color.FgBlack, color.Bold)
	warningBadge   = color.New(color.FgYellow, color.Bold)
	errorBadge     = color.New(color.BgRed, color.FgWhite, color.Bold)
	infoBadge      = color.New(color.FgCyan, color.Bold)
	debugBadge     = color.New(color.FgMagenta)

	// Text colors
	successText = color.New(color.FgGreen, color.Bold)
	warningText = color.New(color.FgYellow)
	errorText   = color.New(color.FgRed)
	infoText    = color.New(color.FgCyan)
	mutedText   = color.New(color.FgHiBlack)
	accentText  = color.New(color.FgMagenta, color.Bold)

	// Special colors
	moneyGreen = color.New(color.FgHiGreen, color.Bold)
	neonPink   = color.New(color.FgHiMagenta, color.Bold)
	neonBlue   = color.New(color.FgHiCyan, color.Bold)

	// Method colors
	methodPOST   = color.New(color.BgHiMagenta, color.FgBlack, color.Bold)
	methodGET    = color.New(color.BgHiCyan, color.FgBlack, color.Bold)
	methodPUT    = color.New(color.BgHiYellow, color.FgBlack, color.Bold)
	methodDELETE = color.New(color.BgHiRed, color.FgBlack, color.Bold)
)

// ══════════════════════════════════════════════════════════════════════════════
// STATUS BADGES
// ══════════════════════════════════════════════════════════════════════════════

// PrintSuccess logs a successful request with green styling.
// Format: [200 OK] message
func PrintSuccess(status int, msg string) {
	successBadge.Printf(" %d OK ", status)
	fmt.Print(" ")
	successText.Println(msg)
}

// PrintSwitching logs a key failover with warning styling.
// Format: ⚠️ [SWITCHING] fromKey → toKey
func PrintSwitching(fromKey, toKey string) {
	fmt.Print("⚠️  ")
	warningBadge.Print("[SWITCHING]")
	fmt.Print(" ")
	mutedText.Print(maskKeyShort(fromKey))
	warningText.Print(" → ")
	accentText.Println(maskKeyShort(toKey))
}

// PrintDeadKey logs when a key is marked as dead.
// Format: 💀 [DEAD KEY] key marked as dead (reason)
func PrintDeadKey(key string, reason string) {
	fmt.Print("💀 ")
	errorBadge.Print(" DEAD KEY ")
	fmt.Print(" ")
	errorText.Print(maskKeyShort(key))
	mutedText.Printf(" marked as dead (%s)\n", reason)
}

// PrintRouterInfo logs general router information.
// Format: [ROUTER] message
func PrintRouterInfo(msg string) {
	infoBadge.Print("[ROUTER]")
	fmt.Print(" ")
	infoText.Println(msg)
}

// PrintChaChing logs the money saved message in bright green.
// Format: 💸 CHA-CHING! You saved $X.XX on this request. Total Saved: $X.XX
func PrintChaChing(saved, total string) {
	moneyGreen.Print("💸 CHA-CHING! ")
	fmt.Print("You saved ")
	moneyGreen.Print(saved)
	fmt.Print(" on this request. Total Saved: ")
	moneyGreen.Println(total)
}

// PrintCacheHit logs a cache hit with lightning styling.
// Format: ⚡ CACHE HIT | key:xxxx...xxxx | 0ms
func PrintCacheHit(cacheKey string, latency time.Duration) {
	neonBlue.Print("⚡ CACHE HIT ")
	fmt.Print("| key:")
	mutedText.Print(maskKeyShort(cacheKey))
	fmt.Print(" | ")
	successText.Printf("%dms\n", latency.Milliseconds())
}

// ══════════════════════════════════════════════════════════════════════════════
// REQUEST LOGGING
// ══════════════════════════════════════════════════════════════════════════════

// PrintRequest logs a request with styled output.
// Color-codes status, method, and latency for quick visual parsing.
func PrintRequest(method, path string, status int, latency time.Duration, keyUsed string) {
	// Timestamp
	mutedText.Printf("%s ", time.Now().Format("15:04:05"))

	// Method badge
	printMethodBadge(method)
	fmt.Print(" ")

	// Path
	fmt.Printf("%-30s ", truncatePath(path, 30))

	// Status badge
	printStatusBadge(status)
	fmt.Print(" ")

	// Latency with color gradient
	printLatency(latency)
	fmt.Print(" ")

	// Key used (masked)
	if keyUsed != "" {
		mutedText.Printf("key:%s", maskKeyShort(keyUsed))
	}

	fmt.Println()
}

// printMethodBadge prints the HTTP method with appropriate color.
func printMethodBadge(method string) {
	switch method {
	case "POST":
		methodPOST.Printf(" %s ", method)
	case "GET":
		methodGET.Printf(" %s ", method)
	case "PUT":
		methodPUT.Printf(" %s ", method)
	case "DELETE":
		methodDELETE.Printf(" %s ", method)
	default:
		debugBadge.Printf(" %s ", method)
	}
}

// printStatusBadge prints the status code with appropriate color.
func printStatusBadge(status int) {
	switch {
	case status >= 200 && status < 300:
		successBadge.Printf(" %d ", status)
	case status >= 300 && status < 400:
		infoBadge.Printf(" %d ", status)
	case status >= 400 && status < 500:
		warningBadge.Printf(" %d ", status)
	default:
		errorBadge.Printf(" %d ", status)
	}
}

// printLatency prints latency with color gradient.
// Green: < 100ms, Yellow: < 500ms, Red: >= 500ms
func printLatency(latency time.Duration) {
	ms := latency.Milliseconds()
	latencyStr := fmt.Sprintf("%4dms", ms)

	switch {
	case ms < 100:
		successText.Print(latencyStr)
	case ms < 500:
		warningText.Print(latencyStr)
	default:
		errorText.Print(latencyStr)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// UTILITY FUNCTIONS
// ══════════════════════════════════════════════════════════════════════════════

// maskKeyShort returns a short masked version of an API key.
// Format: xxxx...xxxx
func maskKeyShort(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

// truncatePath truncates a path to maxLen characters.
func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return path[:maxLen-3] + "..."
}

// ══════════════════════════════════════════════════════════════════════════════
// STARTUP MESSAGES
// ══════════════════════════════════════════════════════════════════════════════

// PrintStartupInfo prints styled server startup information.
func PrintStartupInfo(host string, port int, activeKeys int, strategy string) {
	fmt.Println()
	infoBadge.Print("[ROUTER]")
	fmt.Print(" Server starting on ")
	neonBlue.Printf("http://%s:%d\n", host, port)

	infoBadge.Print("[ROUTER]")
	fmt.Print(" Active keys: ")
	if activeKeys > 0 {
		successText.Printf("%d", activeKeys)
	} else {
		errorText.Printf("%d")
	}
	fmt.Print(" | Strategy: ")
	accentText.Println(strategy)

	fmt.Println()
	printEndpoints()
}

// printEndpoints prints the available API endpoints.
func printEndpoints() {
	mutedText.Println("  ┌─────────────────────────────────────────────────────────┐")
	mutedText.Print("  │ ")
	methodPOST.Print(" POST ")
	fmt.Print(" /v1/chat/completions ")
	mutedText.Print("  Chat completion (OpenAI-compatible)")
	mutedText.Println(" │")
	
	mutedText.Print("  │ ")
	methodGET.Print(" GET  ")
	fmt.Print(" /v1/models           ")
	mutedText.Print("  List available models            ")
	mutedText.Println(" │")
	
	mutedText.Print("  │ ")
	methodGET.Print(" GET  ")
	fmt.Print(" /health              ")
	mutedText.Print("  Health check                     ")
	mutedText.Println(" │")
	
	mutedText.Println("  └─────────────────────────────────────────────────────────┘")
	fmt.Println()
}

// PrintShutdown prints a styled shutdown message.
func PrintShutdown() {
	fmt.Println()
	warningBadge.Print("[SHUTDOWN]")
	warningText.Println(" Graceful shutdown initiated...")
}

// PrintGoodbye prints a styled goodbye message.
func PrintGoodbye() {
	successBadge.Print(" OK ")
	fmt.Print(" ")
	successText.Println("Server stopped. Goodbye! 👋")
}
