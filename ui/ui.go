package ui

import (
	"fmt"
	"goTODO/anytype"
	"image/color"
	"log/slog"
	"sort"
	"sync"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type Invalidator interface {
	Invalidate()
}

type State struct {
	Mu             sync.Mutex
	Tasks          []anytype.Task
	Err            error
	Done           bool
	Loading        bool
	RefreshBtn     widget.Clickable
	CurrentTab     int // 0: Ingoing, 1: Completed
	Tab0Btn        widget.Clickable
	Tab1Btn        widget.Clickable
	SelectedTaskID string
	BackBtn        widget.Clickable
	TaskClickables []widget.Clickable
	LinkClickables []widget.Clickable
	CurrentView    string // "tasks", "details"
}

func SortTasks(tasks []anytype.Task) {
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].DueDate.IsZero() {
			return false
		}
		if tasks[j].DueDate.IsZero() {
			return true
		}
		return tasks[i].DueDate.Before(tasks[j].DueDate)
	})
}

func FetchTasks(client *anytype.Client, s *State, w Invalidator) {
	s.Mu.Lock()
	s.Loading = true
	s.Mu.Unlock()
	w.Invalidate()

	spaceID, _, err := client.GetFirstSpaceID()
	if err != nil {
		slog.Error("FetchTasks: failed to get space ID", "error", err)
		UpdateState(s, nil, err, w)
		return
	}

	typeID, err := client.DiscoverTaskTypeID(spaceID)
	if err != nil {
		slog.Error("FetchTasks: failed to discover task type", "space_id", spaceID, "error", err)
		UpdateState(s, nil, err, w)
		return
	}

	tasks, err := client.FetchTasks(spaceID, typeID)
	if err != nil {
		slog.Error("FetchTasks: failed to fetch tasks", "space_id", spaceID, "type_id", typeID, "error", err)
		UpdateState(s, nil, err, w)
		return
	}
	SortTasks(tasks)

	if len(tasks) > 0 {
		client.Notify("goTODO", fmt.Sprintf("Tasks loaded: %d items", len(tasks)))
	}
	UpdateState(s, tasks, nil, w)
}

func UpdateState(s *State, tasks []anytype.Task, err error, w Invalidator) {
	s.Mu.Lock()
	s.Tasks = tasks
	s.TaskClickables = make([]widget.Clickable, len(tasks))
	s.Err = err
	s.Done = true
	s.Loading = false
	s.Mu.Unlock()
	w.Invalidate()
}

func NavigateToTask(client *anytype.Client, s *State, taskID string, w Invalidator) {
	s.Mu.Lock()
	s.SelectedTaskID = taskID
	s.CurrentView = "details"

	// Pre-initialize link clickables if links are already known
	var links []string
	for _, t := range s.Tasks {
		if t.ID == taskID {
			links = t.Links
			break
		}
	}
	s.LinkClickables = make([]widget.Clickable, len(links))
	s.Mu.Unlock()

	go func() {
		spaceID, _, err := client.GetFirstSpaceID()
		if err != nil {
			slog.Error("NavigateToTask: failed to get space ID", "task_id", taskID, "error", err)
			return
		}
		md, err := client.FetchObjectDetails(spaceID, taskID)
		if err != nil {
			slog.Error("NavigateToTask: failed to fetch details", "task_id", taskID, "error", err)
			return
		}
		cleanedMd := anytype.CleanMarkdown(md)

		s.Mu.Lock()
		targetIdx := -1
		for i, t := range s.Tasks {
			if t.ID == taskID {
				targetIdx = i
				break
			}
		}

		if targetIdx == -1 {
			// Add temporary stub if link not in task list
			newTask := anytype.Task{
				ID:        taskID,
				Name:      "Linked Task (" + taskID + ")",
				Markdown:  cleanedMd,
				LinkNames: make(map[string]string),
			}
			s.Tasks = append(s.Tasks, newTask)
			targetIdx = len(s.Tasks) - 1
		} else {
			s.Tasks[targetIdx].Markdown = cleanedMd
		}

		// Resolve names for all links in the target task
		targetLinks := s.Tasks[targetIdx].Links
		s.LinkClickables = make([]widget.Clickable, len(targetLinks))
		s.Mu.Unlock()
		w.Invalidate() // Initial redraw with markdown content

		for _, linkID := range targetLinks {
			name, err := client.ResolveTaskName(spaceID, linkID)
			if err == nil {
				s.Mu.Lock()
				// Re-verify targetIdx in case s.Tasks changed during resolve
				for i, t := range s.Tasks {
					if t.ID == taskID {
						if s.Tasks[i].LinkNames == nil {
							s.Tasks[i].LinkNames = make(map[string]string)
						}
						s.Tasks[i].LinkNames[linkID] = name
						break
					}
				}
				s.Mu.Unlock()
				w.Invalidate() // Redraw as each link is resolved
			} else {
				slog.Warn("NavigateToTask: failed to resolve link name", "link_id", linkID, "parent_task_id", taskID, "error", err)
			}
		}
	}()
}

func Loop(w *app.Window, s *State, client *anytype.Client) error {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))

	// Initialize state
	s.Mu.Lock()
	if s.CurrentView == "" {
		s.CurrentView = "tasks"
	}
	s.Mu.Unlock()

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

			// Handle Events
			if s.RefreshBtn.Clicked(gtx) {
				go FetchTasks(client, s, w)
			}
			if s.BackBtn.Clicked(gtx) {
				s.Mu.Lock()
				s.CurrentView = "tasks"
				s.SelectedTaskID = ""
				s.Mu.Unlock()
			}
			if s.Tab0Btn.Clicked(gtx) {
				s.Mu.Lock()
				s.CurrentTab = 0
				s.Mu.Unlock()
			}
			if s.Tab1Btn.Clicked(gtx) {
				s.Mu.Lock()
				s.CurrentTab = 1
				s.Mu.Unlock()
			}

			layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							s.Mu.Lock()
							titleText := "goTODO"
							if s.SelectedTaskID != "" {
								titleText = "Task Details"
							}
							s.Mu.Unlock()
							title := material.H4(th, titleText)
							title.Color = color.NRGBA{R: 0x3f, G: 0x51, B: 0xb5, A: 0xff}
							return layout.UniformInset(unit.Dp(16)).Layout(gtx, title.Layout)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								s.Mu.Lock()
								showBack := s.SelectedTaskID != ""
								loading := s.Loading
								s.Mu.Unlock()

								if showBack {
									return material.Button(th, &s.BackBtn, "Back").Layout(gtx)
								}

								btnText := "Refresh"
								if loading {
									btnText = "..."
								}
								btn := material.Button(th, &s.RefreshBtn, btnText)
								if loading {
									gtx = gtx.Disabled()
								}
								return btn.Layout(gtx)
							})
						}),
					)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					s.Mu.Lock()
					selectedID := s.SelectedTaskID
					s.Mu.Unlock()

					if selectedID != "" {
						return showTaskDetails(gtx, th, s, client, w)
					}
					return showTaskList(gtx, th, s, &list, client, w)
				}),
			)

			e.Frame(gtx.Ops)
		}
	}
}

func showTaskList(gtx layout.Context, th *material.Theme, s *State, list *layout.List, client *anytype.Client, w Invalidator) layout.Dimensions {
	s.Mu.Lock()
	if !s.Done && s.Loading {
		s.Mu.Unlock()
		return material.Body1(th, "Loading tasks from Anytype...").Layout(gtx)
	}
	if s.Err != nil {
		errStr := s.Err.Error()
		s.Mu.Unlock()
		return material.Body1(th, "Error: "+errStr).Layout(gtx)
	}

	// Capture state to avoid holding lock during layout and potential deadlocks
	tasks := s.Tasks
	clickables := s.TaskClickables
	currentTab := s.CurrentTab
	tab0Btn := &s.Tab0Btn
	tab1Btn := &s.Tab1Btn

	var filteredTasks []anytype.Task
	var filteredIndices []int
	for i, t := range tasks {
		if (currentTab == 1 && t.IsCompleted) || (currentTab == 0 && !t.IsCompleted) {
			filteredTasks = append(filteredTasks, t)
			filteredIndices = append(filteredIndices, i)
		}
	}
	s.Mu.Unlock()

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{}.Layout(gtx,
				layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
					isActive := currentTab == 0
					btn := material.Button(th, tab0Btn, "Ingoing")
					if isActive {
						btn.Background = color.NRGBA{R: 0x3f, G: 0x51, B: 0xb5, A: 0xff}
					} else {
						btn.Background = color.NRGBA{R: 0xbb, G: 0xbb, B: 0xbb, A: 0xff}
					}
					return layout.UniformInset(unit.Dp(4)).Layout(gtx, btn.Layout)
				}),
				layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
					isActive := currentTab == 1
					btn := material.Button(th, tab1Btn, "Completed")
					if isActive {
						btn.Background = color.NRGBA{R: 0x3f, G: 0x51, B: 0xb5, A: 0xff}
					} else {
						btn.Background = color.NRGBA{R: 0xbb, G: 0xbb, B: 0xbb, A: 0xff}
					}
					return layout.UniformInset(unit.Dp(4)).Layout(gtx, btn.Layout)
				}),
			)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if len(filteredTasks) == 0 {
				msg := "No ingoing tasks."
				if currentTab == 1 {
					msg = "No completed tasks."
				}
				return material.Body1(th, msg).Layout(gtx)
			}

			return list.Layout(gtx, len(filteredTasks), func(gtx layout.Context, i int) layout.Dimensions {
				realIdx := filteredIndices[i]
				if clickables[realIdx].Clicked(gtx) {
					NavigateToTask(client, s, tasks[realIdx].ID, w)
				}

				return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return material.Clickable(gtx, &clickables[realIdx], func(gtx layout.Context) layout.Dimensions {
						t := filteredTasks[i]
						var taskColor color.NRGBA
						prefix := "[ ] "
						if t.IsCompleted {
							prefix = "[x] "
						}

						if !t.DueDate.IsZero() && i == 0 && !t.IsCompleted {
							taskColor = color.NRGBA{R: 0xdb, G: 0x44, B: 0x37, A: 0xff} // Red
							prefix = "(!) " + prefix
						} else if t.IsCompleted {
							taskColor = color.NRGBA{R: 0x88, G: 0x88, B: 0x88, A: 0xff} // Gray
						} else {
							taskColor = color.NRGBA{A: 0xff}
						}

						content := prefix + t.Name
						if t.Status != "" {
							content += " [" + t.Status + "]"
						}
						if !t.DueDate.IsZero() {
							content += " (" + t.DueDate.Format("Jan 02") + ")"
						}

						lbl := material.Body1(th, content)
						lbl.Color = taskColor
						return lbl.Layout(gtx)
					})
				})
			})
		}),
	)
}

func showTaskDetails(gtx layout.Context, th *material.Theme, s *State, client *anytype.Client, w Invalidator) layout.Dimensions {
	s.Mu.Lock()
	var task anytype.Task
	for _, t := range s.Tasks {
		if t.ID == s.SelectedTaskID {
			task = t
			break
		}
	}
	// Ensure clickables match links
	if len(s.LinkClickables) != len(task.Links) {
		s.LinkClickables = make([]widget.Clickable, len(task.Links))
	}
	s.Mu.Unlock()

	return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.H6(th, task.Name).Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				status := "Status: " + task.Status
				if status == "Status: " {
					status = "Status: (Not set)"
				}
				return material.Body2(th, status).Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Body1(th, "Details:").Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				md := task.Markdown
				if md == "" {
					md = "(Loading details...)"
				}
				return material.Body2(th, md).Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if len(task.Links) == 0 {
					return layout.Dimensions{}
				}
				return material.Body1(th, "Related Objects (Links):").Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				var children []layout.FlexChild
				for i, linkID := range task.Links {
					i := i
					linkID := linkID
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if s.LinkClickables[i].Clicked(gtx) {
							NavigateToTask(client, s, linkID, w)
						}
						// Use resolved name if available, fallback to ID
						label := linkID
						s.Mu.Lock()
						if name, ok := task.LinkNames[linkID]; ok && name != "" {
							label = name
						}
						s.Mu.Unlock()

						btn := material.Button(th, &s.LinkClickables[i], "Go to: "+label)
						return layout.UniformInset(unit.Dp(4)).Layout(gtx, btn.Layout)
					}))
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			}),
		)
	})
}
