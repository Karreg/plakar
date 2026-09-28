package tui

import (
	"strconv"
	"strings"

	"github.com/alecthomas/chroma/formatters"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	"github.com/charmbracelet/lipgloss"
)

func renderFilePane(m *tuiModel) string {
	if m.fileInfo == nil {
		if m.loading {
			return "Loading file..."
		}
		return "No file selected"
	}
	var out strings.Builder
	out.WriteString(titleStyle.Render(m.fileInfo.Path) + "\n")
	metadata := m.fileInfo.Mode.String() + "  " + formatSize(m.fileInfo.Size) + "  " + m.fileInfo.ModTime.Local().Format("2006-01-02 15:04:05")
	if m.fileInfo.ContentType != "" {
		metadata += "  " + m.fileInfo.ContentType
	}
	if m.fileInfo.Chunks > 0 {
		metadata += "  " + formatChunkCount(m.fileInfo.Chunks)
	}
	out.WriteString(mutedStyle.Render(metadata) + "\n")
	if m.fileWarning != "" {
		out.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#f2cc8f")).Render(m.fileWarning) + "\n")
	}
	if m.fileContent == "" {
		return out.String()
	}
	lines := strings.Split(m.fileContent, "\n")
	available := max(1, m.height-6)
	start := min(m.fileOffset, max(0, len(lines)-available))
	end := min(start+available, len(lines))
	for _, line := range lines[start:end] {
		out.WriteString(truncate(line, max(1, m.width)) + "\n")
	}
	return out.String()
}

func formatChunkCount(chunks uint64) string {
	if chunks == 1 {
		return "1 chunk"
	}
	return strconv.FormatUint(chunks, 10) + " chunks"
}

func highlightFile(name, text string) string {
	lexer := lexers.Match(name)
	if lexer == nil {
		return text
	}
	iterator, err := lexer.Tokenise(nil, text)
	if err != nil {
		return text
	}
	var rendered strings.Builder
	if err := formatters.TTY256.Format(&rendered, styles.Dracula, iterator); err != nil {
		return text
	}
	return rendered.String()
}
