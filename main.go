package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/skratchdot/open-golang/open"
	calendar "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

var day = 24 * time.Hour

type appKeyMap struct {
	nextEvent     key.Binding
	acceptEvent   key.Binding
	declineEvent  key.Binding
	openInBrowser key.Binding
}

var appKeys = &appKeyMap{
	nextEvent: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "next interesting"),
	),
	acceptEvent: key.NewBinding(
		key.WithKeys("A"),
		key.WithHelp("A", "accept event"),
	),
	declineEvent: key.NewBinding(
		key.WithKeys("D"),
		key.WithHelp("D", "decline event"),
	),
	openInBrowser: key.NewBinding(
		key.WithKeys("o"),
		key.WithHelp("o", "open in browser"),
	),
}

type model struct {
	date   time.Time
	events []*eventItem

	eventsList list.Model
}

func initModel(offset int) model {
	now := time.Now()
	_, tzOffset := now.Zone()
	date := now.Add(time.Duration(tzOffset) * time.Second).Truncate(day).Add(-time.Duration(tzOffset) * time.Second).Add(time.Duration(offset) * day)

	delegate := list.NewDefaultDelegate()
	delegate.UpdateFunc = func(msg tea.Msg, m *list.Model) tea.Cmd {
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
					if ev.attendeeStatus == "needsAction" || len(ev.conflictsWith) > 0 {
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
				return m.NewStatusMessage("open " + ev.Summary)
			}
		}

		return nil
	}

	eventsList := list.New(nil, delegate, 0, 0)
	eventsList.Title = date.Format("2006-01-02")
	// eventsList.SetSpinner(spinner.Dot) // requires padding left value of 3
	eventsList.SetShowStatusBar(false)
	eventsList.StartSpinner() // required here
	eventsList.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			appKeys.nextEvent,
			appKeys.acceptEvent,
			appKeys.declineEvent,
			appKeys.openInBrowser,
		}
	}

	return model{
		date:       date,
		eventsList: eventsList,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.eventsList.StartSpinner(),
		m.loadEvents,
	)
}

var appStyle = lipgloss.NewStyle().Margin(1, 2)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{}

	switch msg := msg.(type) {
	case eventsLoadedMsg:
		m.events = msg.events

		m.eventsList.StopSpinner()

		listItems := make([]list.Item, len(msg.events))
		for i, ev := range msg.events {
			listItems[i] = ev
		}

		cmds = append(cmds, m.eventsList.SetItems(listItems))

	case eventUpdatedMsg:
		m.eventsList.StopSpinner()

	case tea.WindowSizeMsg:
		h, v := appStyle.GetFrameSize()
		m.eventsList.SetSize(msg.Width-h, msg.Height-v)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd

	// m.eventsList.StartSpinner()
	m.eventsList, cmd = m.eventsList.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	s := ""
	s += appStyle.Render(m.eventsList.View())
	return s
}

type eventsLoadedMsg struct {
	events []*eventItem
}

type eventUpdatedMsg struct {
	rawEvent *calendar.Event
}

func (m model) _loadEvents() tea.Msg {
	time.Sleep(3 * time.Second)
	return eventsLoadedMsg{
		events: []*eventItem{
			{
				Event: &calendar.Event{
					Summary: "test",
				},
			},
		},
	}
}

func updateEventStatus(ev *eventItem, status string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		client, err := calendar.NewService(ctx, option.WithHTTPClient(oauthClient))
		if err != nil {
			log.Fatalf("Unable to retrieve Sheets client: %v", err)
		}

		for _, a := range ev.Attendees {
			if a.Self {
				a.ResponseStatus = status
				ev.attendeeStatus = status
				break
			}
		}

		rawEv, err := client.Events.Patch("primary", ev.Id, &calendar.Event{
			Attendees: ev.Attendees,
		}).Do()
		if err != nil {
			log.Fatalf("%#v", err)
		}

		// TODO: handle this
		return eventUpdatedMsg{rawEvent: rawEv}
	}
}

func (m model) loadEvents() tea.Msg {
	ctx := context.Background()

	client, err := calendar.NewService(ctx, option.WithHTTPClient(oauthClient))
	if err != nil {
		log.Fatalf("Unable to retrieve Calendar client: %v", err)
	}

	eventsListResult, err := client.Events.List("primary").
		ShowDeleted(false).
		SingleEvents(true).
		TimeMin(m.date.Format(time.RFC3339)).
		TimeMax(m.date.Add(1 * day).Format(time.RFC3339)).
		OrderBy("startTime").
		Do()
	if err != nil {
		log.Fatalf("%+v", err)
	}

	events := make([]*eventItem, 0, len(eventsListResult.Items))
	for _, it := range eventsListResult.Items {
		if it.Start.DateTime == "" || it.End.DateTime == "" {
			continue
		}

		event := eventItem{Event: it}
		event.start, err = time.Parse(time.RFC3339, it.Start.DateTime)
		if err != nil {
			log.Fatalf("%s: %v", it.Summary, err)
		}
		event.end, err = time.Parse(time.RFC3339, it.End.DateTime)
		if err != nil {
			log.Fatalf("%s: %v", it.Summary, err)
		}

		event.attendeeStatus = "unknown"
		for _, a := range it.Attendees {
			if a.Self {
				event.attendeeStatus = a.ResponseStatus
				break
			}
		}
		if event.attendeeStatus == "unknown" && it.Creator.Self {
			event.attendeeStatus = "accepted"
		}

		events = append(events, &event)
	}

	for _, ev := range events {
		if ev.isDeclined() {
			continue
		}
		for _, ev2 := range events {
			if ev2.isDeclined() {
				continue
			}
			if ev.Id == ev2.Id {
				continue
			}

			if ev.intersectWith(ev2) {
				ev.conflictsWith = append(ev.conflictsWith, ev2)
			}
		}
	}

	return eventsLoadedMsg{
		events: events,
	}
}

type eventItem struct {
	*calendar.Event
	start          time.Time
	end            time.Time
	attendeeStatus string
	conflictsWith  []*eventItem
}

func (e *eventItem) Title() string {
	return statusMarks[e.attendeeStatus] + " " + e.Summary
}

func (e *eventItem) Description() string {
	description := e.start.Format("15:04") + "-" + e.end.Format("15:04")
	if len(e.conflictsWith) > 0 {
		conflictNames := make([]string, len(e.conflictsWith))
		for i := range e.conflictsWith {
			conflictNames[i] = e.conflictsWith[i].Summary
		}
		description += " ! conflicts: " + strings.Join(conflictNames, ",")
	}
	return description
}

func (e *eventItem) FilterValue() string {
	return e.Summary
}

var _ list.DefaultItem = (*eventItem)(nil)

func (e *eventItem) intersectWith(e2 *eventItem) bool {
	return !(e.end.Unix() <= e2.start.Unix() || e2.end.Unix() <= e.start.Unix())
}

func (e *eventItem) isDeclined() bool {
	return e.attendeeStatus == "declined"
}

var statusMarks = map[string]string{
	"accepted":    "✔",
	"declined":    "✖",
	"needsAction": "?",
}

var oauthClient *http.Client

func main() {
	var offset int
	flag.IntVar(&offset, "offset", 0, "offset number of days")
	flag.Parse()

	var err error
	oauthClient, err = getGoogleOAuthClient("credentials.json", []string{calendar.CalendarEventsScope})
	if err != nil {
		log.Fatal(err)
	}

	prog := tea.NewProgram(initModel(offset))
	err = prog.Start()
	if err != nil {
		log.Fatal(err)
	}
}
