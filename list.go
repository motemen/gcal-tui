package main

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/skratchdot/open-golang/open"
)

type eventsListStyles struct {
	list.DefaultItemStyles
	Accepted    lipgloss.Style
	Declined    lipgloss.Style
	NeedsAction lipgloss.Style
	Conflict    lipgloss.Style
}

type eventsListDelegate struct {
	Styles eventsListStyles
}

var _ list.ItemDelegate = (*eventsListDelegate)(nil)

// Height implements list.ItemDelegate
func (d *eventsListDelegate) Height() int {
	return 2
}

// based on DefaultDelegate.Render
func (d *eventsListDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	var (
		title, desc  string
		matchedRunes []int
		s            = &d.Styles
	)

	selectedEvent := m.SelectedItem().(*eventItem)

	ev := item.(*eventItem)

	mark := "●"
	switch ev.attendeeStatus {
	case "accepted":
		mark = s.Accepted.Render("✓")
	case "declined":
		mark = s.Declined.Render("✖")
	case "needsAction", "tentative":
		mark = s.NeedsAction.Render("●")
	}

	timeRange := ev.start.Format("15:04") + "-" + ev.end.Format("15:04")

	conflicts := ""
	conflictNames := make([]string, len(ev.conflictsWith))
	for i := range ev.conflictsWith {
		conflictNames[i] = ev.conflictsWith[i].Summary
	}
	if len(conflictNames) > 0 {
		conflicts = "!"
	}

	if m.Width() <= 0 {
		// short-circuit
		return
	}

	// Conditions
	var (
		isSelected  = index == m.Index()
		emptyFilter = m.FilterState() == list.Filtering && m.FilterValue() == ""
		isFiltered  = m.FilterState() == list.Filtering || m.FilterState() == list.FilterApplied
	)

	if isFiltered && index < len(m.VisibleItems()) {
		// Get indices of matched characters
		matchedRunes = m.MatchesForItem(index)
	}

	if emptyFilter {
		title = s.DimmedTitle.Render(title)
		desc = s.DimmedDesc.Render(desc)
	} else if isSelected && m.FilterState() != list.Filtering {
		if isFiltered {
			// Highlight matches
			unmatched := s.SelectedTitle.Inline(true)
			matched := unmatched.Copy().Inherit(s.FilterMatch)
			title = lipgloss.StyleRunes(title, matchedRunes, matched, unmatched)
			title = s.SelectedTitle.Render(mark + " " + title)
		} else {
			// title = s.SelectedTitle.Render(mark + " " + s.SelectedTitle.Copy().Border(lipgloss.Border{}).Padding(0).Render(ev.Summary))
			title = s.SelectedTitle.Render(mark + " " + s.SelectedTitle.Inline(true).Render(ev.Summary))
			// desc = s.SelectedDesc.Render(timeRange + " " + conflicts)
			desc = s.NormalDesc.Render(timeRange + " " + s.Conflict.Inline(true).Render(conflicts))
		}
	} else {
		if isFiltered {
			// Highlight matches
			unmatched := s.NormalTitle.Inline(true)
			matched := unmatched.Copy().Inherit(s.FilterMatch)
			title = lipgloss.StyleRunes(title, matchedRunes, matched, unmatched)
			title = s.NormalTitle.Render(mark + " " + title)
		} else {
			isConflicting := false
			for _, c := range selectedEvent.conflictsWith {
				if ev.Id == c.Id {
					isConflicting = true
					break
				}
			}
			// title = s.NormalTitle.Render(mark + " " + s.NormalTitle.Copy().Border(lipgloss.Border{}).Padding(0).Render(ev.Summary))
			if ev.attendeeStatus == "declined" {
				title = s.NormalTitle.Render(mark + " " + s.Declined.Inline(true).Render(ev.Summary))
			} else {
				title = s.NormalTitle.Render(mark + " " + s.NormalTitle.Inline(true).Render(ev.Summary))
			}
			if isConflicting {
				// FIXME
				// desc = s.Conflict.Copy().Padding(s.NormalDesc.GetPadding()).Render(timeRange + " " + conflicts)
				desc = s.NormalDesc.Render(timeRange + " " + s.Conflict.Inline(true).Render(conflicts))
			} else {
				desc = s.NormalDesc.Render(timeRange + " " + conflicts)
			}
		}
	}

	fmt.Fprintf(w, "%s\n%s", title, desc)
}

// Spacing implements list.ItemDelegate
func (*eventsListDelegate) Spacing() int {
	return 0
}

// Update implements list.ItemDelegate
func (*eventsListDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	ev, ok := m.SelectedItem().(*eventItem)
	if !ok {
		return nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, appKeys.nextEvent):
			// focus next interesting event
			items := m.Items()
			events := make([]*eventItem, len(items))
			for i, it := range items {
				events[i] = it.(*eventItem)
			}

			i := m.Index()
			for j := i + 1; j < i+len(events); j++ {
				ev := events[j%len(events)]
				if ev.attendeeStatus == "needsAction" || ev.attendeeStatus == "tentative" || len(ev.conflictsWith) > 0 {
					m.Select(j % len(events))
					break
				}
			}
			return nil

		case key.Matches(msg, appKeys.acceptEvent):
			return tea.Batch(
				m.StartSpinner(),
				updateEventStatus(ev, "accepted"),
			)

		case key.Matches(msg, appKeys.declineEvent):
			return tea.Batch(
				m.StartSpinner(),
				updateEventStatus(ev, "declined"),
			)

		case key.Matches(msg, appKeys.openInBrowser):
			open.Start(ev.HtmlLink)
			return m.NewStatusMessage("open " + ev.Summary + " in browser")
		}
	}

	return nil
}
