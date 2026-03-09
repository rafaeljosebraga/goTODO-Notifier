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
	MenuOpen       bool
	TitleBtn       widget.Clickable
	SettingsBtn    widget.Clickable
	TasksBtn       widget.Clickable
	MockMBtn       widget.Clickable
	CurrentView    string // "tasks", "settings", "mockM"
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
		UpdateState(s, nil, err, w)
		return
	}

	typeID, err := client.DiscoverTaskTypeID(spaceID)
	if err != nil {
		UpdateState(s, nil, err, w)
		return
	}

	tasks, err := client.FetchTasks(spaceID, typeID)
	SortTasks(tasks)

	if err == nil && len(tasks) > 0 {
		client.Notify("goTODO", fmt.Sprintf("Tasks loaded: %d items", len(tasks)))
	}
	UpdateState(s, tasks, err, w)
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
			if s.TitleBtn.Clicked(gtx) {
				s.Mu.Lock()
				s.MenuOpen = !s.MenuOpen
				s.Mu.Unlock()
			}
			if s.TasksBtn.Clicked(gtx) {
				s.Mu.Lock()
				s.CurrentView = "tasks"
				s.SelectedTaskID = ""
				s.MenuOpen = false
				s.Mu.Unlock()
			}
			if s.SettingsBtn.Clicked(gtx) {
				s.Mu.Lock()
				s.CurrentView = "settings"
				s.SelectedTaskID = ""
				s.MenuOpen = false
				s.Mu.Unlock()
			}
			if s.MockMBtn.Clicked(gtx) {
				s.Mu.Lock()
				s.CurrentView = "mockM"
				s.SelectedTaskID = ""
				s.MenuOpen = false
				s.Mu.Unlock()
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
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										s.Mu.Lock()
										title := "goTODO"
										if s.SelectedTaskID != "" {
											title = "Task Detail"
										}
										s.Mu.Unlock()

										return material.Clickable(gtx, &s.TitleBtn, func(gtx layout.Context) layout.Dimensions {
											return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													t := material.H4(th, title)
													t.Color = color.NRGBA{R: 0x3f, G: 0x51, B: 0xb5, A: 0xff}
													return t.Layout(gtx)
												}),
												layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													symbol := "▾"
													s.Mu.Lock()
													if s.MenuOpen {
														symbol = "▴"
													}
													s.Mu.Unlock()
													t := material.H4(th, symbol)
													t.Color = color.NRGBA{R: 0x3f, G: 0x51, B: 0xb5, A: 0xff}
													return t.Layout(gtx)
												}),
											)
										})
									})
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										s.Mu.Lock()
										loading := s.Loading
										isTasks := s.CurrentView == "tasks" && s.SelectedTaskID == ""
										s.Mu.Unlock()

										if !isTasks {
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
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							s.Mu.Lock()
							open := s.MenuOpen
							s.Mu.Unlock()
							if !open {
								return layout.Dimensions{}
							}
							return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											btn := material.Button(th, &s.TasksBtn, "Tasks")
											return btn.Layout(gtx)
										})
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											btn := material.Button(th, &s.SettingsBtn, "Settings")
											return btn.Layout(gtx)
										})
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											btn := material.Button(th, &s.MockMBtn, "mockM")
											return btn.Layout(gtx)
										})
									}),
								)
							})
						}),
					)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					s.Mu.Lock()
					selectedID := s.SelectedTaskID
					view := s.CurrentView
					s.Mu.Unlock()

					if selectedID != "" {
						return showTaskDetails(gtx, th, s, client, w)
					}

					switch view {
					case "settings":
						return showSettings(gtx, th, s, client, w)
					case "mockM":
						return showMockM(gtx, th, s, client, w)
					default:
						return showTaskList(gtx, th, s, &list, client, w)
					}
				}),
			)

			e.Frame(gtx.Ops)
		}
	}
}

func showMockM(gtx layout.Context, th *material.Theme, s *State, client *anytype.Client, w Invalidator) layout.Dimensions {
	return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.H6(th, "MockM Menu (Feature Demo)").Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Body1(th, "This is a demonstration of the UI's flexibility.").Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Body2(th, "Adding this menu required only adding a button to the state, a navigation handler, and this content function.").Layout(gtx)
			}),
		)
	})
}

func showSettings(gtx layout.Context, th *material.Theme, s *State, client *anytype.Client, w Invalidator) layout.Dimensions {
	return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.H6(th, "Anytype API Configuration").Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Body1(th, "Base URL:").Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Body2(th, client.BaseURL).Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Body1(th, "API Key (Masked):").Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				masked := "****************"
				if len(client.APIKey) > 8 {
					masked = client.APIKey[:4] + "..." + client.APIKey[len(client.APIKey)-4:]
				}
				return material.Body2(th, masked).Layout(gtx)
			}),
		)
	})
}

func showTaskList(gtx layout.Context, th *material.Theme, s *State, list *layout.List, client *anytype.Client, w Invalidator) layout.Dimensions {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if !s.Done && s.Loading {
		return material.Body1(th, "Loading tasks from Anytype...").Layout(gtx)
	}
	if s.Err != nil {
		return material.Body1(th, "Error: "+s.Err.Error()).Layout(gtx)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{}.Layout(gtx,
				layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
					isActive := s.CurrentTab == 0
					btn := material.Button(th, &s.Tab0Btn, "Ingoing")
					if isActive {
						btn.Background = color.NRGBA{R: 0x3f, G: 0x51, B: 0xb5, A: 0xff}
					} else {
						btn.Background = color.NRGBA{R: 0xbb, G: 0xbb, B: 0xbb, A: 0xff}
					}
					return layout.UniformInset(unit.Dp(4)).Layout(gtx, btn.Layout)
				}),
				layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
					isActive := s.CurrentTab == 1
					btn := material.Button(th, &s.Tab1Btn, "Completed")
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
			var filteredTasks []anytype.Task
			var filteredIndices []int
			for i, t := range s.Tasks {
				if (s.CurrentTab == 1 && t.IsCompleted) || (s.CurrentTab == 0 && !t.IsCompleted) {
					filteredTasks = append(filteredTasks, t)
					filteredIndices = append(filteredIndices, i)
				}
			}

			if len(filteredTasks) == 0 {
				msg := "No ingoing tasks."
				if s.CurrentTab == 1 {
					msg = "No completed tasks."
				}
				return material.Body1(th, msg).Layout(gtx)
			}

			return list.Layout(gtx, len(filteredTasks), func(gtx layout.Context, i int) layout.Dimensions {
				realIdx := filteredIndices[i]
				if s.TaskClickables[realIdx].Clicked(gtx) {
					taskID := s.Tasks[realIdx].ID
					go func() {
						s.Mu.Lock()
						s.SelectedTaskID = taskID
						s.Mu.Unlock()

						spaceID, _, _ := client.GetFirstSpaceID()
						md, _ := client.FetchObjectDetails(spaceID, taskID)

						s.Mu.Lock()
						for i := range s.Tasks {
							if s.Tasks[i].ID == taskID {
								s.Tasks[i].Markdown = md
							}
						}
						s.Mu.Unlock()
						w.Invalidate()
					}()
				}

				return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return material.Clickable(gtx, &s.TaskClickables[realIdx], func(gtx layout.Context) layout.Dimensions {
						t := filteredTasks[i]
						var taskColor color.NRGBA
						prefix := "[ ] "
						if t.IsCompleted {
							prefix = "[x] "
						}

						if !t.DueDate.IsZero() && i == 0 && !t.IsCompleted {
							taskColor = color.NRGBA{R: 0xdb, G: 0x44, B: 0x37, A: 0xff} // Red
							prefix = "🔥 " + prefix
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
				content := ""
				for _, link := range task.Links {
					content += "- " + link + "\n"
				}
				return material.Body2(th, content).Layout(gtx)
			}),
		)
	})
}
