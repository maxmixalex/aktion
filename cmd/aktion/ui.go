package main

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

type tableCell struct {
	Text  string
	Style string
	URL   string
}

type metricCard struct {
	Label string
	Value string
	Tone  string
	Hint  string
}

type UI struct {
	profile colorprofile.Profile
	styles  map[string]lipgloss.Style
	links   bool
}

var ui *UI

func newUI(colorMode, linkMode string) (*UI, error) {
	colorMode = strings.ToLower(strings.TrimSpace(colorMode))
	if colorMode == "" {
		colorMode = "auto"
	}
	if colorMode != "auto" && colorMode != "always" && colorMode != "never" {
		return nil, fmt.Errorf("invalid --color value %q; expected auto, always, or never", colorMode)
	}

	linkMode = strings.ToLower(strings.TrimSpace(linkMode))
	if linkMode == "" {
		linkMode = "auto"
	}
	if linkMode != "auto" && linkMode != "always" && linkMode != "never" {
		return nil, fmt.Errorf("invalid --links value %q; expected auto, always, or never", linkMode)
	}

	var profile colorprofile.Profile
	switch colorMode {
	case "always":
		profile = colorprofile.TrueColor
	case "never":
		profile = colorprofile.NoTTY
	default:
		profile = colorprofile.Detect(os.Stdout, os.Environ())
	}

	links := false
	switch linkMode {
	case "always":
		links = true
	case "never":
		links = false
	default:
		links = stdoutIsTTY() &&
			!strings.EqualFold(strings.TrimSpace(os.Getenv("TERM")), "dumb") &&
			os.Getenv("NO_HYPERLINK") == "" &&
			os.Getenv("NO_LINKS") == ""
	}

	styles := map[string]lipgloss.Style{
		"title":      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.BrightCyan),
		"subtitle":   lipgloss.NewStyle().Foreground(lipgloss.BrightBlack),
		"header":     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.BrightWhite),
		"accent":     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.BrightCyan),
		"good":       lipgloss.NewStyle().Foreground(lipgloss.BrightGreen),
		"bad":        lipgloss.NewStyle().Foreground(lipgloss.BrightRed),
		"warn":       lipgloss.NewStyle().Foreground(lipgloss.BrightYellow),
		"muted":      lipgloss.NewStyle().Foreground(lipgloss.BrightBlack),
		"info":       lipgloss.NewStyle().Foreground(lipgloss.BrightBlue),
		"bold":       lipgloss.NewStyle().Bold(true),
		"dim":        lipgloss.NewStyle().Faint(true),
		"blue":       lipgloss.NewStyle().Foreground(lipgloss.Blue),
		"border":     lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		"goodBadge":  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Black).Background(lipgloss.BrightGreen).Padding(0, 1),
		"badBadge":   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.White).Background(lipgloss.Red).Padding(0, 1),
		"warnBadge":  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Black).Background(lipgloss.BrightYellow).Padding(0, 1),
		"infoBadge":  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Black).Background(lipgloss.BrightCyan).Padding(0, 1),
		"mutedBadge": lipgloss.NewStyle().Bold(true).Foreground(lipgloss.White).Background(lipgloss.Color("238")).Padding(0, 1),
	}
	return &UI{profile: profile, styles: styles, links: links}, nil
}

func stdoutIsTTY() bool {
	st, err := os.Stdout.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

func (u *UI) link(label, target string) string {
	if u == nil || !u.links || label == "" || target == "" {
		return label
	}
	parsed, err := url.Parse(target)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
		return label
	}
	if strings.ContainsAny(target, "a
") {
		return label
	}
	return "]8;;" + target + "\" + label + "]8;;\"
}

func visibleWidth(s string) int { return lipgloss.Width(s) }

func (u *UI) downsample(s string) string {
	if u == nil || s == "" {
		return s
	}
	var buf bytes.Buffer
	w := colorprofile.Writer{Forward: &buf, Profile: u.profile}
	_, _ = w.WriteString(s)
	return buf.String()
}

func (u *UI) renderStyle(kind, s string) string {
	if u == nil || s == "" {
		return s
	}
	style, ok := u.styles[kind]
	if !ok {
		return s
	}
	return u.downsample(style.Render(s))
}

func (u *UI) badge(kind, text string) string {
	style := kind + "Badge"
	if _, ok := u.styles[style]; !ok {
		style = "mutedBadge"
	}
	return u.renderStyle(style, strings.ToUpper(text))
}

func (u *UI) banner(title, subtitle string) {
	line := u.renderStyle("title", title)
	if subtitle != "" {
		line += "  " + u.renderStyle("subtitle", subtitle)
	}
	fmt.Println(line)
	fmt.Println(u.renderStyle("border", strings.Repeat("─", max(32, visibleWidth(title)+visibleWidth(subtitle)+2))))
}

func (u *UI) section(title string) { fmt.Println(u.renderStyle("accent", "◆ "+title)) }

func (u *UI) errorLine(w io.Writer, prefix, message string) {
	if u == nil {
		fmt.Fprintf(w, "%s%s
", prefix, message)
		return
	}
	style := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Red)
	fmt.Fprintln(w, u.downsample(style.Render(prefix))+message)
}

func (u *UI) panel(title, body, tone string) string {
	borderColor := lipgloss.Color("240")
	switch tone {
	case "good":
		borderColor = lipgloss.BrightGreen
	case "bad":
		borderColor = lipgloss.BrightRed
	case "warn":
		borderColor = lipgloss.BrightYellow
	case "info":
		borderColor = lipgloss.BrightCyan
	}
	heading := u.renderStyle("accent", title)
	content := heading
	if body != "" {
		content += "
" + body
	}
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1)
	return u.downsample(style.Render(content))
}

func (u *UI) metrics(cards ...metricCard) string {
	rendered := make([]string, 0, len(cards))
	for _, c := range cards {
		value := u.renderStyle("bold", c.Value)
		if c.Tone != "" {
			value = u.badge(c.Tone, c.Value)
		}
		body := u.renderStyle("muted", c.Label) + "
" + value
		if c.Hint != "" {
			body += "
" + u.renderStyle("muted", c.Hint)
		}
		style := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1).
			Width(22)
		rendered = append(rendered, u.downsample(style.Render(body)))
	}
	if len(rendered) == 0 {
		return ""
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, intersperse(rendered, "  ")...)
}

func intersperse(items []string, sep string) []string {
	if len(items) <= 1 {
		return items
	}
	out := make([]string, 0, len(items)*2-1)
	for i, item := range items {
		if i > 0 {
			out = append(out, sep)
		}
		out = append(out, item)
	}
	return out
}

func (u *UI) table(headers []string, rows [][]tableCell) string {
	if len(headers) == 0 {
		return ""
	}
	widths := make([]int, len(headers))
	for i, h := range headers { widths[i] = visibleWidth(h) }
	for _, row := range rows {
		for i := 0; i < len(row) && i < len(widths); i++ {
			if w := visibleWidth(row[i].Text); w > widths[i] { widths[i] = w }
		}
	}

	border := func(left, mid, right, fill string) string {
		var b strings.Builder
		b.WriteString(left)
		for i, w := range widths {
			b.WriteString(strings.Repeat(fill, w+2))
			if i < len(widths)-1 { b.WriteString(mid) }
		}
		b.WriteString(right)
		return u.renderStyle("border", b.String())
	}

	renderRow := func(cells []tableCell, header bool) string {
		var b strings.Builder
		b.WriteString(u.renderStyle("border", "│"))
		for i := range headers {
			text, kind := "", ""
			if header {
				text, kind = headers[i], "header"
			} else if i < len(cells) {
				text, kind = cells[i].Text, cells[i].Style
			}
			pad := widths[i] - visibleWidth(text)
			if header {
				b.WriteString(" " + u.renderStyle(kind, text) + strings.Repeat(" ", pad) + " ")
			} else {
				styled := u.renderStyle(kind, text)
				if i < len(cells) && cells[i].URL != "" { styled = u.link(styled, cells[i].URL) }
				b.WriteString(" " + styled + strings.Repeat(" ", pad) + " ")
			}
			b.WriteString(u.renderStyle("border", "│"))
		}
		return b.String()
	}

	var b strings.Builder
	b.WriteString(border("╭", "┬", "╮", "─"))
	b.WriteByte('
')
	b.WriteString(renderRow(nil, true))
	b.WriteByte('
')
	b.WriteString(border("├", "┼", "┤", "─"))
	for _, row := range rows {
		b.WriteByte('
')
		b.WriteString(renderRow(row, false))
	}
	b.WriteByte('
')
	b.WriteString(border("╰", "┴", "╯", "─"))
	return b.String()
}

func (u *UI) progressBar(percent float64, width int, tone string) string {
	if width < 4 { width = 4 }
	if percent < 0 { percent = 0 }
	if percent > 100 { percent = 100 }
	filled := int((percent/100.0)*float64(width) + 0.5)
	if filled > width { filled = width }
	empty := width - filled
	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	if tone == "" { tone = "info" }
	return u.renderStyle(tone, bar)
}

func cell(text string) tableCell              { return tableCell{Text: text} }
func styledCell(text, style string) tableCell { return tableCell{Text: text, Style: style} }
func linkedCell(text, style, target string) tableCell {
	return tableCell{Text: text, Style: style, URL: target}
}
func goodCell(text string) tableCell                 { return styledCell(text, "good") }
func badCell(text string) tableCell                  { return styledCell(text, "bad") }
func warnCell(text string) tableCell                 { return styledCell(text, "warn") }
func mutedCell(text string) tableCell                { return styledCell(text, "muted") }
func accentCell(text string) tableCell               { return styledCell(text, "accent") }
func infoCell(text string) tableCell                 { return styledCell(text, "info") }
func linkedAccentCell(text, target string) tableCell { return linkedCell(text, "accent", target) }
func linkedInfoCell(text, target string) tableCell   { return linkedCell(text, "info", target) }
func linkedMutedCell(text, target string) tableCell  { return linkedCell(text, "muted", target) }
