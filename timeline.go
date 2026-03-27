package main

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"golang.org/x/term"
)

var (
	timelineBarStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#50CFFA"))
	timelineConflictStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#AD3252"))
	timelineDeclinedStyle = lipgloss.NewStyle().Strikethrough(true).Foreground(lipgloss.Color("#777777"))
	timelineAcceptedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#2EAD71"))
	timelineDimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#555555"))
)

func printEventsTimeline(date time.Time) error {
	events, err := fetchEventsForDate(date)
	if err != nil {
		return err
	}

	weekday := jaWeekday(date.Weekday())

	if len(events) == 0 {
		fmt.Printf("%s (%s)\n\n  予定はありません\n", date.Format("2006-01-02"), weekday)
		return nil
	}

	// Determine time range
	minTime, maxTime := timeRange(events)

	// Terminal width
	termWidth := 80
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		termWidth = w
	}

	// Name column width: max event name width, capped
	const maxNameWidth = 20
	nameWidth := 0
	for _, ev := range events {
		w := runewidth.StringWidth(ev.Summary)
		if w > nameWidth {
			nameWidth = w
		}
	}
	if nameWidth > maxNameWidth {
		nameWidth = maxNameWidth
	}
	if nameWidth < 4 {
		nameWidth = 4
	}

	// Time label column width: "  10:00-11:00" => ~15
	const timeLabelWidth = 15

	// Bar width = remaining (1 for conflict mark, 2 for spacing around bar)
	barWidth := termWidth - 1 - nameWidth - 2 - timeLabelWidth
	if barWidth < 10 {
		barWidth = 10
	}

	// Print header
	fmt.Printf("%s (%s)\n\n", date.Format("2006-01-02"), weekday)

	// Print time axis header
	printTimeAxis(nameWidth, barWidth, minTime, maxTime)

	// Print each event
	for _, ev := range events {
		printEventBar(ev, nameWidth, barWidth, minTime, maxTime)
	}

	return nil
}

func jaWeekday(w time.Weekday) string {
	names := [...]string{"日", "月", "火", "水", "木", "金", "土"}
	return names[w]
}

func timeRange(events []*eventItem) (minTime, maxTime time.Time) {
	minTime = events[0].Start
	maxTime = events[0].End

	for _, ev := range events {
		if ev.Start.Before(minTime) {
			minTime = ev.Start
		}
		if ev.End.After(maxTime) {
			maxTime = ev.End
		}
	}

	// Round down to hour for minTime, round up for maxTime
	minTime = minTime.Truncate(time.Hour)
	if maxTime.Truncate(time.Hour) != maxTime {
		maxTime = maxTime.Truncate(time.Hour).Add(time.Hour)
	}

	// Ensure at least 1 hour range
	if !maxTime.After(minTime) {
		maxTime = minTime.Add(time.Hour)
	}

	return
}

func timeToPos(t, minTime, maxTime time.Time, barWidth int) int {
	total := maxTime.Sub(minTime).Seconds()
	if total == 0 {
		return 0
	}
	elapsed := t.Sub(minTime).Seconds()
	pos := int(math.Round(elapsed / total * float64(barWidth)))
	if pos < 0 {
		pos = 0
	}
	if pos > barWidth {
		pos = barWidth
	}
	return pos
}

func printTimeAxis(nameWidth, barWidth int, minTime, maxTime time.Time) {
	// Build label and tick lines as rune slices for proper positioning
	labelSlots := make([]byte, barWidth)
	for i := range labelSlots {
		labelSlots[i] = ' '
	}

	tickRunes := make([]string, barWidth)
	for i := range tickRunes {
		tickRunes[i] = " "
	}

	t := minTime
	for !t.After(maxTime) {
		pos := timeToPos(t, minTime, maxTime, barWidth)
		label := t.Format("15")

		if pos < barWidth {
			tickRunes[pos] = "╵"
		}

		// Place label so it starts at pos (left-aligned from tick)
		for i := 0; i < len(label); i++ {
			p := pos + i
			if p >= 0 && p < barWidth {
				labelSlots[p] = label[i]
			}
		}

		t = t.Add(time.Hour)
	}

	padding := strings.Repeat(" ", nameWidth+3) // 1(conflict mark) + nameWidth + 2(spacing)
	fmt.Printf("%s%s\n", padding, string(labelSlots))
	fmt.Printf("%s%s\n", padding, strings.Join(tickRunes, ""))
}

func printEventBar(ev *eventItem, nameWidth, barWidth int, minTime, maxTime time.Time) {
	// Truncate or pad event name
	name := truncateString(ev.Summary, nameWidth)
	pad := nameWidth - runewidth.StringWidth(name)
	if pad < 0 {
		pad = 0
	}
	namePadded := name + strings.Repeat(" ", pad)

	// Calculate bar positions
	startPos := timeToPos(ev.Start, minTime, maxTime, barWidth)
	endPos := timeToPos(ev.End, minTime, maxTime, barWidth)

	// For instant/very short events
	isShort := endPos-startPos < 2
	if endPos <= startPos {
		endPos = startPos + 1
	}

	// Build bar
	bar := make([]string, barWidth)
	for i := range bar {
		bar[i] = "─"
	}

	// Place event block
	if isShort {
		if startPos > 0 {
			bar[startPos-1] = "┨"
		}
		if startPos < barWidth {
			bar[startPos] = "░"
		}
		if startPos+1 < barWidth {
			bar[startPos+1] = "┠"
		}
	} else {
		if startPos > 0 {
			bar[startPos-1] = "┨"
		}
		for i := startPos; i < endPos && i < barWidth; i++ {
			bar[i] = "█"
		}
		if endPos < barWidth {
			bar[endPos] = "┠"
		}
	}

	// Edge markers
	if bar[0] == "─" {
		bar[0] = "╶"
	}
	if bar[barWidth-1] == "─" {
		bar[barWidth-1] = "╴"
	}

	barStr := strings.Join(bar, "")

	// Time label
	var timeLabel string
	if ev.Start.Equal(ev.End) || isShort {
		timeLabel = ev.Start.Format("15:04")
	} else {
		timeLabel = ev.Start.Format("15:04") + "-" + ev.End.Format("15:04")
	}

	// Conflict marker (left side)
	conflictMark := " "
	if len(ev.ConflictsWith) > 0 && !ev.Declined() {
		conflictMark = timelineConflictStyle.Render("!")
	}

	// Style the output
	var nameStyled, barStyled, timeLabelStyled string

	if ev.Declined() {
		nameStyled = timelineDeclinedStyle.Render(namePadded)
		barStyled = timelineDimStyle.Render(barStr)
		timeLabelStyled = timelineDeclinedStyle.Render(timeLabel)
	} else if len(ev.ConflictsWith) > 0 {
		nameStyled = timelineConflictStyle.Render(namePadded)
		barStyled = timelineConflictStyle.Render(barStr)
		timeLabelStyled = timelineConflictStyle.Render(timeLabel)
	} else if ev.Accepted() {
		nameStyled = timelineAcceptedStyle.Render(namePadded)
		barStyled = timelineAcceptedStyle.Render(barStr)
		timeLabelStyled = timelineAcceptedStyle.Render(timeLabel)
	} else {
		nameStyled = timelineBarStyle.Render(namePadded)
		barStyled = timelineBarStyle.Render(barStr)
		timeLabelStyled = timelineBarStyle.Render(timeLabel)
	}

	fmt.Printf("%s%s  %s  %s\n", conflictMark, nameStyled, barStyled, timeLabelStyled)
}

func truncateString(s string, maxWidth int) string {
	w := runewidth.StringWidth(s)
	if w <= maxWidth {
		return s
	}

	result := ""
	currentWidth := 0
	for _, r := range s {
		rw := runewidth.RuneWidth(r)
		if currentWidth+rw > maxWidth-2 { // 2 for ".."
			break
		}
		result += string(r)
		currentWidth += rw
	}
	return result + ".."
}
