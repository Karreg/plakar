package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f2cc8f"))
	mutedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#8d99ae"))
	selectStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#81b29a"))
	valueStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f4f1de"))
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#e07a5f"))
)

func renderView(m *tuiModel) string {
	var body string
	switch m.view {
	case viewDashboard:
		body = renderDashboard(m)
	case viewSnapshots:
		body = renderSnapshots(m)
	case viewBrowser:
		body = renderBrowser(m)
	case viewFile:
		body = renderFilePane(m)
	}

	status := "q quit | r refresh"
	switch m.view {
	case viewDashboard:
		status += " | s snapshots"
	case viewSnapshots:
		status += " | up/down select | left/right page | enter browse | esc dashboard"
	case viewBrowser:
		status += " | up/down select | enter open | backspace parent"
	case viewFile:
		status += " | up/down scroll | pgup/pgdn | esc back"
	}
	if m.loading {
		status = "Loading..."
	}
	if m.err != nil {
		body += "\n\n" + errorStyle.Render(m.err.Error())
		status = "r retry · q quit"
	}
	if m.width > 0 {
		body = lipgloss.NewStyle().Width(m.width).Render(body)
	}
	return body + "\n" + mutedStyle.Render(status)
}

func renderDashboard(m *tuiModel) string {
	var out strings.Builder
	out.WriteString(titleStyle.Render("plakar | repository dashboard"))
	out.WriteString("\n")
	if m.dashboard == nil {
		if m.loading {
			out.WriteString("Loading repository statistics...")
		}
		return out.String()
	}
	data := m.dashboard
	out.WriteString(mutedStyle.Render("Location  ") + data.Location + "\n\n")
	out.WriteString(mutedStyle.Render("Snapshots ") + valueStyle.Render(fmt.Sprintf("%d", data.Total)) + "\n")
	out.WriteString(mutedStyle.Render("Stored    ") + valueStyle.Render(formatSize(data.StorageSize)) + "\n")
	out.WriteString(mutedStyle.Render("Logical   ") + valueStyle.Render(formatSize(int64(data.LogicalSize))) + "\n")
	if data.Efficiency < 0 {
		out.WriteString(mutedStyle.Render("Efficiency ") + valueStyle.Render("n/a") + "\n\n")
	} else {
		out.WriteString(mutedStyle.Render("Efficiency ") + valueStyle.Render(fmt.Sprintf("%.1f%%", data.Efficiency)) + "\n\n")
	}
	out.WriteString(mutedStyle.Render("Snapshots per day | last 30 days") + "\n")
	out.WriteString(renderHistogram(data.SnapshotsPerDay))
	return out.String()
}

func renderHistogram(values []int) string {
	if len(values) == 0 {
		return "No activity\n"
	}
	maximum := 0
	for _, value := range values {
		if value > maximum {
			maximum = value
		}
	}
	if maximum == 0 {
		return mutedStyle.Render("No snapshots in this period") + "\n"
	}
	const levels = 5
	var out strings.Builder
	for row := levels; row > 0; row-- {
		threshold := (maximum*row + levels - 1) / levels
		for _, value := range values {
			if value >= threshold {
				out.WriteByte('#')
			} else {
				out.WriteByte(' ')
			}
		}
		out.WriteByte('\n')
	}
	for index := range values {
		switch index {
		case 0:
			out.WriteByte('1')
		case 9:
			out.WriteByte('1')
		case 19:
			out.WriteByte('2')
		case 29:
			out.WriteByte('3')
		default:
			out.WriteByte(' ')
		}
	}
	out.WriteString("\n")
	return out.String()
}

func renderSnapshots(m *tuiModel) string {
	var out strings.Builder
	out.WriteString(titleStyle.Render("Snapshots") + "\n")
	if m.loading && len(m.snapshots) == 0 {
		return out.String() + "Loading snapshots..."
	}
	if len(m.snapshots) == 0 {
		return out.String() + mutedStyle.Render("No snapshots found")
	}
	out.WriteString(mutedStyle.Render(fmt.Sprintf("%-20s %-12s %9s %9s  %s", "CREATED", "ID", "SIZE", "DURATION", "SOURCE")) + "\n")
	pageStart := m.page * snapshotsPerPage
	pageCount := min(snapshotsPerPage, len(m.snapshots)-pageStart)
	visibleCount := min(pageCount, max(1, m.height-8))
	visibleStart := max(0, min(m.cursor-visibleCount/2, pageCount-visibleCount))
	start := pageStart + visibleStart
	end := start + visibleCount
	for index := start; index < end; index++ {
		item := m.snapshots[index]
		marker := "  "
		style := lipgloss.NewStyle()
		if index-pageStart == m.cursor {
			marker = "> "
			style = selectStyle
		}
		line := fmt.Sprintf("%-20s %-12s %9s %9s  %s",
			item.Timestamp.Local().Format("2006-01-02 15:04:05"), item.ShortID,
			humanize.IBytes(item.Size), item.Duration.Round(time.Second), item.Importer)
		if len(item.Tags) > 0 {
			line += "  " + strings.Join(item.Tags, ",")
		}
		out.WriteString(style.Render(marker+truncate(line, max(1, m.width-2))) + "\n")
	}
	out.WriteString(mutedStyle.Render(fmt.Sprintf("Page %d/%d | %d snapshots", m.page+1, (len(m.snapshots)+snapshotsPerPage-1)/snapshotsPerPage, len(m.snapshots))))
	return out.String()
}

func renderBrowser(m *tuiModel) string {
	var out strings.Builder
	out.WriteString(titleStyle.Render("Snapshot browser") + "\n")
	out.WriteString(mutedStyle.Render(m.dirPath) + "\n\n")
	if m.loading && len(m.entries) == 0 {
		return out.String() + "Loading directory..."
	}
	if len(m.entries) == 0 {
		return out.String() + mutedStyle.Render("Empty directory")
	}
	visibleCount := min(len(m.entries), max(1, m.height-6))
	visibleStart := max(0, min(m.cursor-visibleCount/2, len(m.entries)-visibleCount))
	for index := visibleStart; index < visibleStart+visibleCount; index++ {
		item := m.entries[index]
		marker := "  "
		style := lipgloss.NewStyle()
		if index == m.cursor {
			marker = "> "
			style = selectStyle
		}
		name := item.Name
		if item.IsDir {
			name += "/"
		}
		if item.SymlinkTarget != "" {
			name += " -> " + item.SymlinkTarget
		}
		line := fmt.Sprintf("%-42s %9s %s", name, humanize.IBytes(uint64(max(0, item.Size))), item.ModTime.Local().Format("2006-01-02 15:04"))
		out.WriteString(style.Render(marker+truncate(line, max(1, m.width-2))) + "\n")
	}
	return out.String()
}

func formatSize(size int64) string {
	if size < 0 {
		return "unknown"
	}
	return humanize.IBytes(uint64(size))
}

func truncate(value string, width int) string {
	if width < 1 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 3 {
		return strings.Repeat(".", width)
	}
	return string(runes[:width-3]) + "..."
}
