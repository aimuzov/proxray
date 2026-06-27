// Package ui renders user-facing command output to stdout: status lines,
// server descriptions, and tables. Diagnostics belong in internal/log.
package ui

import (
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/aimuzov/happ-cli/internal/link"
)

var (
	out io.Writer          = os.Stdout
	rnd *lipgloss.Renderer = lipgloss.NewRenderer(os.Stdout)
)

// Palette. lipgloss disables color automatically on non-TTY writers and when
// NO_COLOR is set, so these are safe for piped output.
const (
	colorSuccess = lipgloss.Color("42")  // green
	colorInfo    = lipgloss.Color("39")  // blue
	colorWarn    = lipgloss.Color("214") // amber
	colorError   = lipgloss.Color("196") // red
	colorMuted   = lipgloss.Color("245") // gray
)

// SetOutput redirects ui output and rebuilds the renderer for w. Tests pass a
// bytes.Buffer (a non-TTY writer), which yields the Ascii profile (no ANSI).
func SetOutput(w io.Writer) {
	out = w
	rnd = lipgloss.NewRenderer(w)
	rnd.SetColorProfile(termenv.Ascii)
}

// Success prints a green ✓ status line.
func Success(format string, a ...any) {
	mark := rnd.NewStyle().Foreground(colorSuccess).Bold(true).Render("✓")
	fmt.Fprintf(out, "%s %s\n", mark, fmt.Sprintf(format, a...))
}

// Info prints a neutral, informational status line.
func Info(format string, a ...any) {
	fmt.Fprintln(out, rnd.NewStyle().Foreground(colorInfo).Render(fmt.Sprintf(format, a...)))
}

// Warn prints an amber "warning:" status line.
func Warn(format string, a ...any) {
	label := rnd.NewStyle().Foreground(colorWarn).Bold(true).Render("warning:")
	fmt.Fprintf(out, "%s %s\n", label, fmt.Sprintf(format, a...))
}

// Error prints a red "error:" status line.
func Error(format string, a ...any) {
	label := rnd.NewStyle().Foreground(colorError).Bold(true).Render("error:")
	fmt.Fprintf(out, "%s %s\n", label, fmt.Sprintf(format, a...))
}

// Server prints a one-line description: "Server #1: Tag [proto] addr:port".
func Server(idx int, s link.Server) {
	tag := rnd.NewStyle().Bold(true).Render(s.Tag)
	proto := rnd.NewStyle().Foreground(colorMuted).Render("[" + s.Protocol + "]")
	fmt.Fprintf(out, "Server #%d: %s %s %s:%d\n", idx+1, tag, proto, s.Address, s.Port)
}

// Endpoints prints the local proxy listener block.
func Endpoints(socksPort, httpPort int) {
	fmt.Fprint(out, Table(
		[]string{"PROXY", "ADDRESS"},
		[][]string{
			{"SOCKS5", fmt.Sprintf("127.0.0.1:%d", socksPort)},
			{"HTTP", fmt.Sprintf("127.0.0.1:%d", httpPort)},
		},
	))
}
