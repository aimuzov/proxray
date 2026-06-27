package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

// Table renders a colored, aligned table. Unlike text/tabwriter it accounts for
// ANSI width correctly, so colored headers/cells stay aligned.
func Table(headers []string, rows [][]string) string {
	headerStyle := rnd.NewStyle().Bold(true).Foreground(colorInfo).Padding(0, 1)
	cellStyle := rnd.NewStyle().Padding(0, 1)
	return table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(rnd.NewStyle().Foreground(colorMuted)).
		Headers(headers...).
		Rows(rows...).
		StyleFunc(func(row, _ int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return cellStyle
		}).
		Render() + "\n"
}
