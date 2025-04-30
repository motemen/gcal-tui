package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/motemen/go-nuts/oauth2util"
	"golang.org/x/oauth2/google"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	calendar "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

const programName = "gcal-tui"

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
	jumpToDay     key.Binding
}

func (a appKeyMap) helpKeys() []key.Binding {
	return []key.Binding{
		a.nextEvent,
		a.acceptEvent,
		a.declineEvent,
		a.openInBrowser,
		a.gotoToday,
		a.reload,
		a.nextDay,
		a.prevDay,
		a.jumpToDay,
	}
}

var appKeys = &appKeyMap{
	nextEvent: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "next todo"),
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
		key.WithHelp("→", "next day"),
	),
	prevDay: key.NewBinding(
		key.WithKeys("left"),
		key.WithHelp("←", "prev day"),
	),
	jumpToDay: key.NewBinding(
		key.WithKeys("ctrl+t"),
		key.WithHelp("ctrl+t", "jump to day"),
	),
}

type model struct {
	date   time.Time
	events []*eventItem

	// Default view
	eventsList list.Model

	// Day view
	inputDate textinput.Model

	uiMode uiMode

	errorMessage string
}

type uiMode int

const (
	uiModeDefault uiMode = iota
	uiModeInputDate
)

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

func initModel() model {
	date := today()

	delegate := &eventsListDelegate{
		Styles: styles,
	}

	eventsList := list.New(nil, delegate, 0, 0)
	eventsList.Styles.Title.Background(lipgloss.Color("#50CFFA"))
	eventsList.Title = date.Format("2006-01-02")
	eventsList.SetSpinner(thinDotSpinner)
	eventsList.SetShowStatusBar(false)
	eventsList.StartSpinner() // required here
	eventsList.AdditionalShortHelpKeys = appKeys.helpKeys
	eventsList.AdditionalFullHelpKeys = appKeys.helpKeys
	eventsList.KeyMap.Filter.Unbind()

	t := textinput.New()
	t.Placeholder = "YYYY-MM-DD"
	t.Focus()
	t.CharLimit = 10
	t.Width = 15

	return model{
		date:       date,
		eventsList: eventsList,
		inputDate:  t,
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

func (m model) enterJumpToDayMode() (model, tea.Cmd) {
	m.errorMessage = ""
	m.uiMode = uiModeInputDate
	m.inputDate.SetValue(m.date.Format("2006-01-02"))
	return m, textinput.Blink
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

	case nonFatalErrorMsg:
		m.errorMessage = msg.errorMessage

	case tea.WindowSizeMsg:
		h, v := appStyle.GetFrameSize()
		m.eventsList.SetSize(msg.Width-h, msg.Height-v)

	case tea.KeyMsg:
		switch m.uiMode {
		case uiModeDefault:
			model, cmd := m.handleDefaultModeKeyMsg(msg)
			if cmd != nil {
				return model, cmd
			}

		case uiModeInputDate:
			if msg.String() == "enter" {
				date, err := time.Parse("2006-01-02", m.inputDate.Value())
				if err != nil {
					m.errorMessage = err.Error()
				} else {
					m.uiMode = uiModeDefault
					return m.reloadEvents(date)
				}
			}
		}
	}

	var cmd tea.Cmd

	switch m.uiMode {
	case uiModeDefault:
		// m.eventsList.StartSpinner()
		m.eventsList, cmd = m.eventsList.Update(msg)
		cmds = append(cmds, cmd)

	case uiModeInputDate:
		m.inputDate, cmd = m.inputDate.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m model) handleDefaultModeKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		return m.reloadEvents(m.date.Add(+1 * day))

	case key.Matches(msg, appKeys.prevDay):
		return m.reloadEvents(m.date.Add(-1 * day))

	case key.Matches(msg, appKeys.jumpToDay):
		return m.enterJumpToDayMode()
	}

	return m, nil
}

func (m model) View() string {
	if m.uiMode == uiModeInputDate {
		return appStyle.Render("Date: " + m.inputDate.View())
	}

	return appStyle.Render(m.eventsList.View())
}

type eventsLoadedMsg struct {
	events []*eventItem
}

type eventUpdatedMsg struct {
	rawEvent *calendar.Event
}

type nonFatalErrorMsg struct {
	errorMessage string
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
				ev.AttendeeStatus = status
				break
			}
		}

		rawEv, err := client.Events.Patch("primary", ev.Id, &calendar.Event{
			Attendees: ev.Attendees,
		}).Do()
		if err != nil {
			log.Fatalf("%#v", err)
		}

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
		event.Start, err = time.Parse(time.RFC3339, it.Start.DateTime)
		if err != nil {
			log.Fatalf("%s: %v", it.Summary, err)
		}
		event.End, err = time.Parse(time.RFC3339, it.End.DateTime)
		if err != nil {
			log.Fatalf("%s: %v", it.Summary, err)
		}

		event.AttendeeStatus = "unknown"
		for _, a := range it.Attendees {
			if a.Self {
				event.AttendeeStatus = a.ResponseStatus
				break
			}
		}
		if event.AttendeeStatus == "unknown" && it.Creator.Self {
			event.AttendeeStatus = "accepted"
		}

		events = append(events, &event)
	}

	for _, ev := range events {
		if ev.IsDeclined() {
			continue
		}
		for _, ev2 := range events {
			if ev2.IsDeclined() {
				continue
			}
			if ev.Id == ev2.Id {
				continue
			}

			if ev.intersectWith(ev2) {
				ev.ConflictsWith = append(ev.ConflictsWith, ev2)
			}
		}
	}

	return eventsLoadedMsg{
		events: events,
	}
}

type eventItem struct {
	*calendar.Event
	Start          time.Time
	End            time.Time
	AttendeeStatus string
	ConflictsWith  []*eventItem
}

func (e *eventItem) FilterValue() string {
	return e.Summary
}

func (e *eventItem) intersectWith(e2 *eventItem) bool {
	return !(e.End.Unix() <= e2.Start.Unix() || e2.End.Unix() <= e.Start.Unix())
}

func (e *eventItem) IsAccepted() bool {
	return e.AttendeeStatus == "accepted"
}

func (e *eventItem) IsDeclined() bool {
	return e.AttendeeStatus == "declined"
}

func (e *eventItem) String() string {
	return fmt.Sprintf("%s-%s %s",
		e.Start.Format("15:04"),
		e.End.Format("15:04"),
		e.Summary,
	)
}

var oauthClient *http.Client

func main() {
	confDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}

	credentialsFile := filepath.Join(confDir, programName, "credentials.json")
	formatStr := ""
	flag.StringVar(&credentialsFile, "credentials", credentialsFile, "`path` to credentials.json")
	flag.StringVar(&formatStr, "format", "", "Go template for event output (non-interactive mode)")
	flag.Parse()

	b, err := os.ReadFile(credentialsFile)
	if err != nil {
		log.Print(err)
		log.Fatal("Unable to read credentials file -- check README.md to get started.")
	}

	oauth2Config, err := google.ConfigFromJSON(b, calendar.CalendarEventsScope)
	if err != nil {
		log.Fatal(err)
	}

	oauthClient, err = (&oauth2util.Config{
		OAuth2Config: oauth2Config,
		Name:         programName,
	}).CreateOAuth2Client(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	if formatStr != "" {
		// 非インタラクティブ: テンプレート出力
		err := printEventsWithTemplate(formatStr)
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	prog := tea.NewProgram(initModel())
	err = prog.Start()
	if err != nil {
		log.Fatal(err)
	}
}

func printEventsWithTemplate(formatStr string) error {
	events, err := fetchTodayEvents()
	if err != nil {
		return err
	}

	funcMap := template.FuncMap{
		"formatTime": func(t time.Time, layout string) string {
			return t.Format(layout)
		},
	}

	tmpl, err := template.New("events").Funcs(funcMap).Parse(formatStr)
	if err != nil {
		return fmt.Errorf("template parse error: %w", err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, events)
	if err != nil {
		return fmt.Errorf("template execute error: %w", err)
	}
	fmt.Print(buf.String())
	return nil
}

func fetchTodayEvents() ([]*eventItem, error) {
	return fetchEventsForDate(today())
}

func fetchEventsForDate(date time.Time) ([]*eventItem, error) {
	ctx := context.Background()
	client, err := calendar.NewService(ctx, option.WithHTTPClient(oauthClient))
	if err != nil {
		return nil, err
	}
	eventsListResult, err := client.Events.List("primary").
		ShowDeleted(false).
		SingleEvents(true).
		TimeMin(date.Format(time.RFC3339)).
		TimeMax(date.Add(1 * day).Format(time.RFC3339)).
		OrderBy("startTime").
		Do()
	if err != nil {
		return nil, err
	}
	events := make([]*eventItem, 0, len(eventsListResult.Items))
	for _, it := range eventsListResult.Items {
		if it.Start.DateTime == "" || it.End.DateTime == "" {
			continue
		}
		event := eventItem{Event: it}
		event.Start, err = time.Parse(time.RFC3339, it.Start.DateTime)
		if err != nil {
			return nil, err
		}
		event.End, err = time.Parse(time.RFC3339, it.End.DateTime)
		if err != nil {
			return nil, err
		}
		event.AttendeeStatus = "unknown"
		for _, a := range it.Attendees {
			if a.Self {
				event.AttendeeStatus = a.ResponseStatus
				break
			}
		}
		if event.AttendeeStatus == "unknown" && it.Creator.Self {
			event.AttendeeStatus = "accepted"
		}
		events = append(events, &event)
	}

	// Detect conflicts
	for _, ev := range events {
		if ev.IsDeclined() {
			continue
		}
		for _, ev2 := range events {
			if ev2.IsDeclined() {
				continue
			}
			if ev.Id == ev2.Id {
				continue
			}
			if ev.intersectWith(ev2) {
				ev.ConflictsWith = append(ev.ConflictsWith, ev2)
			}
		}
	}
	return events, nil
}
