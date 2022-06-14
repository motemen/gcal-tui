package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/motemen/go-nuts/oauth2util"
	"golang.org/x/oauth2/google"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	calendar "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

var day = 24 * time.Hour

type appKeyMap struct {
	nextEvent     key.Binding
	acceptEvent   key.Binding
	declineEvent  key.Binding
	openInBrowser key.Binding
	gotoToday     key.Binding
	reload        key.Binding
	nextDay       key.Binding
	prevDay       key.Binding
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
	gotoToday: key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "today"),
	),
	reload: key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("R", "reload"),
	),
	nextDay: key.NewBinding(
		key.WithKeys("right"),
		key.WithHelp("right", "next day"),
	),
	prevDay: key.NewBinding(
		key.WithKeys("left"),
		key.WithHelp("left", "prev day"),
	),
}

type model struct {
	date   time.Time
	events []*eventItem

	eventsList list.Model
}

var thinDotSpinner = spinner.Spinner{
	Frames: []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"},
	FPS:    time.Second / 10,
}

var styles = eventsListStyles{
	DefaultItemStyles: list.NewDefaultItemStyles(),

	Accepted:    lipgloss.NewStyle().Foreground(lipgloss.Color("#2EAD71")),
	Declined:    lipgloss.NewStyle().Strikethrough(true).Foreground(lipgloss.Color("#777777")),
	NeedsAction: lipgloss.NewStyle().Foreground(lipgloss.Color("#D6CF69")),
	Conflict:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#AD3252")),
}

func init() {
	styles.DefaultItemStyles.SelectedTitle.
		Foreground(lipgloss.Color("#50CFFA")).BorderLeftForeground(lipgloss.Color("#50CFFA"))
}

func today() time.Time {
	now := time.Now()
	_, tzOffset := now.Zone()
	date := now.Add(time.Duration(tzOffset) * time.Second).Truncate(day).Add(-time.Duration(tzOffset) * time.Second)
	return date
}

func initModel(offset int) model {
	date := today().Add(time.Duration(offset) * day)

	delegate := &eventsListDelegate{
		Styles: styles,
	}

	eventsList := list.New(nil, delegate, 0, 0)
	eventsList.Styles.Title.Background(lipgloss.Color("#50CFFA"))
	eventsList.Title = date.Format("2006-01-02")
	eventsList.SetSpinner(thinDotSpinner)
	eventsList.SetShowStatusBar(false)
	eventsList.StartSpinner() // required here
	eventsList.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			appKeys.nextEvent,
			appKeys.acceptEvent,
			appKeys.declineEvent,
			appKeys.openInBrowser,
			appKeys.nextDay,
			appKeys.prevDay,
		}
	}

	return model{
		date:       date,
		eventsList: eventsList,
	}
}

func (m model) reloadEvents(date time.Time) (model, tea.Cmd) {
	m.date = date
	m.eventsList.Title = m.date.Format("2006-01-02")
	return m, tea.Batch(
		m.eventsList.StartSpinner(),
		m.eventsList.SetItems(nil),
		m.loadEvents,
	)
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

		switch {
		case key.Matches(msg, appKeys.gotoToday):
			return m.reloadEvents(today())

		case key.Matches(msg, appKeys.reload):
			return m.reloadEvents(m.date)

		case key.Matches(msg, appKeys.nextDay):
			return m.reloadEvents(m.date.Add(1 * day))

		case key.Matches(msg, appKeys.prevDay):
			return m.reloadEvents(m.date.Add(-1 * day))
		}
	}

	var cmd tea.Cmd

	// m.eventsList.StartSpinner()
	m.eventsList, cmd = m.eventsList.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	return appStyle.Render(m.eventsList.View())
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

func (e *eventItem) FilterValue() string {
	return e.Summary
}

func (e *eventItem) intersectWith(e2 *eventItem) bool {
	return !(e.end.Unix() <= e2.start.Unix() || e2.end.Unix() <= e.start.Unix())
}

func (e *eventItem) isDeclined() bool {
	return e.attendeeStatus == "declined"
}

var oauthClient *http.Client

func main() {
	var offset int
	var credentials string
	flag.IntVar(&offset, "offset", 0, "offset number of days")
	flag.StringVar(&credentials, "credentials", "credentials.json", "`path` to credentials.json")
	flag.Parse()

	b, err := os.ReadFile(credentials)
	if err != nil {
		log.Fatal(err)
	}

	oauth2Config, err := google.ConfigFromJSON(b, calendar.CalendarEventsScope)
	if err != nil {
		log.Fatal(err)
	}

	oauthClient, err = (&oauth2util.Config{
		OAuth2Config: oauth2Config,
		Name:         "tui-gcal",
	}).CreateOAuth2Client(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	prog := tea.NewProgram(initModel(offset))
	err = prog.Start()
	if err != nil {
		log.Fatal(err)
	}
}
