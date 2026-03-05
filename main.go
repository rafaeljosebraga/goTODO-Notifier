package main

import (
	"fmt"
	"goTODO/anytype"
	"goTODO/config"
	"goTODO/ui"
	"log"
	"os"
	"time"

	"gioui.org/app"
	"gioui.org/unit"
)

func main() {
	cfg := config.Load()
	log.Printf("Starting goTODO UI (Anytype CLI Port: 31012)")

	anytypeClient := anytype.NewClient(cfg.APIKey)
	appState := &ui.State{}

	go func() {
		w := new(app.Window)
		w.Option(app.Title("goTODO"))
		w.Option(app.Size(unit.Dp(500), unit.Dp(600)))

		// Start fetching tasks in background
		go ui.FetchTasks(anytypeClient, appState, w)

		// Start periodic reminders
		go startReminderLoop(anytypeClient, appState)

		if err := ui.Loop(w, appState, anytypeClient); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func startReminderLoop(client *anytype.Client, s *ui.State) {
	ticker := time.NewTicker(5 * time.Second)
	for range ticker.C {
		s.Mu.Lock()
		var priorityTask *anytype.Task
		for _, t := range s.Tasks {
			if !t.DueDate.IsZero() && t.DueDate.After(time.Now().Add(-24*time.Hour)) {
				priorityTask = &t
				break
			}
		}
		s.Mu.Unlock()

		if priorityTask != nil {
			client.Notify("⏰ Task Reminder", fmt.Sprintf("Next up: %s\nDue: %s",
				priorityTask.Name, priorityTask.DueDate.Format("Jan 02")))
		}
	}
}
