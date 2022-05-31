package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"golang.org/x/tools/container/intsets"

	survey "github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	calendar "google.golang.org/api/calendar/v3"
)

var day = 24 * time.Hour

type eventItem struct {
	*calendar.Event
	start          time.Time
	end            time.Time
	attendeeStatus string
}

var statusMarks = map[string]string{
	"accepted":    "✔",
	"declined":    "✖",
	"needsAction": "?",
}

var statusColor = map[string][]color.Attribute{
	"accepted":    {color.FgHiBlue, color.Bold},
	"declined":    {color.Faint},
	"needsAction": {},
}

const markConflict = "💥"

func main() {
	ctx := context.Background()

	var offset int
	flag.IntVar(&offset, "offset", 0, "offset number of days")
	flag.Parse()

	client, err := calendar.NewService(ctx)
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", err)
	}

	targetDay := time.Now().Truncate(day).Add(time.Duration(offset) * day)

	eventsListResult, err := client.Events.List("primary").
		ShowDeleted(false).
		SingleEvents(true).
		TimeMin(targetDay.Format(time.RFC3339)).
		TimeMax(targetDay.Add(1 * day).Format(time.RFC3339)).
		OrderBy("startTime").
		Do()
	if err != nil {
		log.Fatalf("%+v", err)
	}

	events := make([]eventItem, 0, len(eventsListResult.Items))
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

		events = append(events, event)
	}

	var (
		eventsIn  = map[int][]int{}
		eventsOut = map[int][]int{}
		times     = intsets.Sparse{}
	)

	fmt.Printf("# %s\n", targetDay.Format("2006-01-02"))

	for i, ev := range events {
		if ev.attendeeStatus == "declined" {
			continue
		}
		start := int(ev.start.Unix())
		end := int(ev.end.Unix())
		eventsIn[start] = append(eventsIn[start], i)
		eventsOut[end] = append(eventsOut[end], i)
		times.Insert(int(ev.start.Unix()))
		times.Insert(int(ev.end.Unix()))
	}

	// event index -> set of overbooked event indices
	overbookedWith := map[int]*intsets.Sparse{}

	conflicts := [][]int{}

	set := intsets.Sparse{}
	var t int
	for times.TakeMin(&t) {
		// TODO: declined は虫
		for _, i := range eventsIn[t] {
			set.Insert(i)
		}
		for _, i := range eventsOut[t] {
			set.Remove(i)
		}

		if set.Len() > 1 {
			log.Printf("%d events overbooked: %s~", set.Len(), time.Unix(int64(t), 0).Format("15:04"))

			for _, i := range set.AppendTo(nil) {
				if overbookedWith[i] == nil {
					overbookedWith[i] = &intsets.Sparse{}
				}

				overbookedWith[i].UnionWith(&set)
			}

			conflicts = append(conflicts, set.AppendTo(nil))
		}
	}

	for i, ev := range events {
		overbookInfo := ""
		var timeColor color.Attribute
		if events[i].attendeeStatus != "declined" && overbookedWith[i] != nil {
			ss := []string{}
			for _, j := range overbookedWith[i].AppendTo(nil) {
				if i == j {
					continue
				}
				if events[j].attendeeStatus == "declined" {
					continue
				}
				ss = append(ss, events[j].Summary)
			}
			if len(ss) > 0 {
				overbookInfo = color.New(color.FgYellow).Sprint(markConflict + " " + strings.Join(ss, ", "))
				timeColor = color.FgHiRed
			}
		}
		fmt.Printf(
			"%s [%2d] %s %s %v\n",
			color.New(timeColor).Sprint(ev.start.Format("15:04")+"-"+ev.end.Format("15:04")),
			i,
			color.New(statusColor[ev.attendeeStatus]...).Sprint(statusMarks[ev.attendeeStatus]),
			color.New(statusColor[ev.attendeeStatus]...).Sprint(ev.Summary),
			overbookInfo,
		)
	}

	for _, conflict := range conflicts {
		names := make([]string, len(conflict))
		for i, ei := range conflict {
			names[i] = events[ei].Summary
		}

		log.Println("Conflicts: " + strings.Join(names, ","))

		var ans []int
		survey.AskOne(
			&survey.MultiSelect{
				Options: names,
			},
			&ans,
		)
	}

	// TODO: コンフリクトのあるもの・未定のものから出す
	// * [A]ccept and decline other conficts
	// * [a]ccept
	// * [d]ecline
	// * [o]pen in browser
}
