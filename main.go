package main

import (
	"fmt"
	"goTODO/anytype"
	"goTODO/config"
	"image/color"
	"log"
	"os"
	"sort"
	"sync"
	"time"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

type state struct {
	mu    sync.Mutex
	tasks []anytype.Task
	err   error
	done  bool
}

func main() {
	cfg := config.Load()
	log.Printf("Starting goTODO UI (Anytype CLI Port: 31012)")

	anytypeClient := anytype.NewClient(cfg.APIKey)
	appState := &state{}

	go func() {
		w := new(app.Window)
		w.Option(app.Title("goTODO"))
		w.Option(app.Size(unit.Dp(500), unit.Dp(600)))

		// Start fetching tasks in background
		go fetchTasks(anytypeClient, appState, w)

		// Start periodic reminders
		go startReminderLoop(anytypeClient, appState)

		if err := loop(w, appState); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func fetchTasks(client *anytype.Client, s *state, w *app.Window) {
	spaceID, _, err := client.GetFirstSpaceID()
	if err != nil {
		updateState(s, nil, err, w)
		return
	}

	typeID, err := client.DiscoverTaskTypeID(spaceID)
	if err != nil {
		updateState(s, nil, err, w)
		return
	}

	tasks, err := client.FetchTasks(spaceID, typeID)
	// Sort tasks by due date (soonest first, empty dates last)
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].DueDate.IsZero() {
			return false
		}
		if tasks[j].DueDate.IsZero() {
			return true
		}
		return tasks[i].DueDate.Before(tasks[j].DueDate)
	})

	if err == nil && len(tasks) > 0 {
		client.Notify("goTODO", fmt.Sprintf("Tasks loaded: %d items", len(tasks)))
	}
	updateState(s, tasks, err, w)
}

func startReminderLoop(client *anytype.Client, s *state) {
	ticker := time.NewTicker(5 * time.Second)
	for range ticker.C {
		s.mu.Lock()
		var priorityTask *anytype.Task
		for _, t := range s.tasks {
			if !t.DueDate.IsZero() && t.DueDate.After(time.Now().Add(-24*time.Hour)) {
				priorityTask = &t
				break
			}
		}
		s.mu.Unlock()

		if priorityTask != nil {
			client.Notify("⏰ Task Reminder", fmt.Sprintf("Next up: %s\nDue: %s",
				priorityTask.Name, priorityTask.DueDate.Format("Jan 02")))
		}
	}
}

func updateState(s *state, tasks []anytype.Task, err error, w *app.Window) {
	s.mu.Lock()
	s.tasks = tasks
	s.err = err
	s.done = true
	s.mu.Unlock()
	w.Invalidate()
}

func loop(w *app.Window, s *state) error {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))

	var ops op.Ops
	var list layout.List
	list.Axis = layout.Vertical

	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			ops.Reset()

			layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					title := material.H4(th, "goTODO Tasks")
					title.Color = color.NRGBA{R: 0x3f, G: 0x51, B: 0xb5, A: 0xff}
					title.Alignment = text.Middle
					return layout.UniformInset(unit.Dp(16)).Layout(gtx, title.Layout)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					s.mu.Lock()
					defer s.mu.Unlock()

					if !s.done {
						return material.Body1(th, "Loading tasks from Anytype...").Layout(gtx)
					}
					if s.err != nil {
						return material.Body1(th, "Error: "+s.err.Error()).Layout(gtx)
					}
					if len(s.tasks) == 0 {
						return material.Body1(th, "No tasks found.").Layout(gtx)
					}

					return list.Layout(gtx, len(s.tasks), func(gtx layout.Context, i int) layout.Dimensions {
						return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							t := s.tasks[i]
							var taskColor color.NRGBA
							prefix := "- "

							// Highlight priority task (first one with a date)
							if !t.DueDate.IsZero() && i == 0 {
								taskColor = color.NRGBA{R: 0xdb, G: 0x44, B: 0x37, A: 0xff} // Red
								prefix = "🔥 "
							} else {
								taskColor = color.NRGBA{A: 0xff}
							}

							content := prefix + t.Name
							if !t.DueDate.IsZero() {
								content += " (" + t.DueDate.Format("Jan 02") + ")"
							}

							lbl := material.Body1(th, content)
							lbl.Color = taskColor
							return lbl.Layout(gtx)
						})
					})
				}),
			)

			e.Frame(gtx.Ops)
		}
	}
}
