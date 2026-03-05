package ui

import (
	"fmt"
	"goTODO/anytype"
	"image/color"
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

type State struct {
	Mu         sync.Mutex
	Tasks      []anytype.Task
	Err        error
	Done       bool
	Loading    bool
	RefreshBtn widget.Clickable
}

func FetchTasks(client *anytype.Client, s *State, w *app.Window) {
	s.Mu.Lock()
	s.Loading = true
	s.Mu.Unlock()
	w.Invalidate()

	spaceID, _, err := client.GetFirstSpaceID()
	if err != nil {
		UpdateState(s, nil, err, w)
		return
	}

	typeID, err := client.DiscoverTaskTypeID(spaceID)
	if err != nil {
		UpdateState(s, nil, err, w)
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
	UpdateState(s, tasks, err, w)
}

func UpdateState(s *State, tasks []anytype.Task, err error, w *app.Window) {
	s.Mu.Lock()
	s.Tasks = tasks
	s.Err = err
	s.Done = true
	s.Loading = false
	s.Mu.Unlock()
	w.Invalidate()
}

func Loop(w *app.Window, s *State, client *anytype.Client) error {
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

			if s.RefreshBtn.Clicked(gtx) {
				go FetchTasks(client, s, w)
			}

			layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							title := material.H4(th, "goTODO Tasks")
							title.Color = color.NRGBA{R: 0x3f, G: 0x51, B: 0xb5, A: 0xff}
							return layout.UniformInset(unit.Dp(16)).Layout(gtx, title.Layout)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								s.Mu.Lock()
								loading := s.Loading
								s.Mu.Unlock()

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
					defer s.Mu.Unlock()

					if !s.Done && s.Loading {
						return material.Body1(th, "Loading tasks from Anytype...").Layout(gtx)
					}
					if s.Err != nil {
						return material.Body1(th, "Error: "+s.Err.Error()).Layout(gtx)
					}
					if len(s.Tasks) == 0 {
						return material.Body1(th, "No tasks found.").Layout(gtx)
					}

					return list.Layout(gtx, len(s.Tasks), func(gtx layout.Context, i int) layout.Dimensions {
						return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							t := s.Tasks[i]
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
