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
		if m.editingFilter {
			status = "up/down choose | tab switch | enter apply | esc cancel | ctrl+c quit"
		} else {
			status += " | f filter | up/down select | left/right page | enter browse | esc dashboard"
		}
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
	return out.String()
}

func renderSnapshots(m *tuiModel) string {
	var out strings.Builder
	out.WriteString(titleStyle.Render("Snapshots") + "\n")
	menuCount := 0
	if m.editingFilter {
		for index, label := range []string{"Perimeter", "Tag"} {
			marker := "  "
			if index == m.filterField {
				marker = "> "
			}
			value := m.filterChoices(index)[m.filterInputs[index]]
			if value == "" {
				value = "All"
			}
			line := fmt.Sprintf("%s%-10s %s", marker, label+":", value)
			if m.width > 0 {
				line = truncate(line, m.width)
			}
			out.WriteString(line)
			out.WriteByte('\n')
		}
		choices := m.filterChoices(m.filterField)
		menuCount = min(len(choices), max(1, min(6, m.height-9)))
		start := max(0, min(m.filterInputs[m.filterField]-menuCount/2, len(choices)-menuCount))
		for index := start; index < start+menuCount; index++ {
			label := choices[index]
			if label == "" {
				label = "All"
			} else if index >= len(m.filterOptions[m.filterField]) {
				label += " (unavailable)"
			}
			marker := "    "
			style := mutedStyle
			if index == m.filterInputs[m.filterField] {
				marker = "  > "
				style = selectStyle
			}
			line := marker + label
			if m.width > 0 {
				line = truncate(line, m.width)
			}
			out.WriteString(style.Render(line))
			out.WriteByte('\n')
		}
		out.WriteString(mutedStyle.Render(fmt.Sprintf("Choices %d-%d/%d", start+1, start+menuCount, len(choices))))
		out.WriteByte('\n')
	} else if m.filter.perimeter != "" || m.filter.tag != "" {
		filters := fmt.Sprintf("Filters: perimeter=%q tag=%q", m.filter.perimeter, m.filter.tag)
		if m.width > 0 {
			filters = truncate(filters, m.width)
		}
		out.WriteString(mutedStyle.Render(filters))
		out.WriteByte('\n')
	}
	if m.loading && len(m.snapshots) == 0 {
		return out.String() + "Loading snapshots..."
	}
	if len(m.snapshots) == 0 {
		if m.filter.perimeter != "" || m.filter.tag != "" {
			return out.String() + mutedStyle.Render("No snapshots match filters")
		}
		return out.String() + mutedStyle.Render("No snapshots found")
	}
	if m.editingFilter && m.height > 0 && m.height <= menuCount+8 {
		return out.String()
	}
	out.WriteString(mutedStyle.Render(fmt.Sprintf("%-20s %-12s %9s %9s  %s", "CREATED", "ID", "SIZE", "DURATION", "SOURCE")) + "\n")
	pageStart := m.page * snapshotsPerPage
	pageCount := min(snapshotsPerPage, len(m.snapshots)-pageStart)
	visibleCount := min(pageCount, max(1, m.height-9))
	if m.editingFilter {
		visibleCount = min(visibleCount, max(1, m.height-8-menuCount))
	}
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
