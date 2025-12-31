package gui

import (
	"fmt"
	"sort"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"PandaBot/internal/job"
	"PandaBot/internal/server"
	"PandaBot/internal/statusMonitor"
)

type memberWidgets struct {
	nameLabel   *widget.Label
	jobLabel    *widget.Label
	hpBar       *widget.ProgressBar
	statusLabel *widget.Label
}

type GUI struct {
	server *server.Server
	app    fyne.App
	window fyne.Window

	partyContainer *fyne.Container
	memberWidgets  map[string]*memberWidgets
}

func NewGUI(srv *server.Server) *GUI {
	a := app.New()
	w := a.NewWindow("PandaBot Server")

	g := &GUI{
		server:        srv,
		app:           a,
		window:        w,
		memberWidgets: make(map[string]*memberWidgets),
	}

	g.partyContainer = container.NewVBox()

	scroll := container.NewVScroll(g.partyContainer)
	scroll.SetMinSize(fyne.NewSize(400, 300))

	w.SetContent(container.NewBorder(
		widget.NewLabelWithStyle("Party Status", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		nil, nil, nil,
		scroll,
	))

	w.Resize(fyne.NewSize(450, 400))

	return g
}

func (g *GUI) Show() {
	go g.refreshLoop()
	g.window.ShowAndRun()
}

func (g *GUI) refreshLoop() {
	ticker := time.NewTicker(1 * time.Second)
	for range ticker.C {
		g.updatePartyInfo()
	}
}

func (g *GUI) updatePartyInfo() {
	sm := g.server.GetStatusMonitor()
	if sm == nil {
		return
	}

	party := sm.GetAllPartyMembers()

	// Get names and sort them for consistent display
	var names []string
	for name := range party {
		names = append(names, name)
	}
	sort.Strings(names)

	// In Fyne, UI updates must happen on the main thread or via g.window.Canvas().Refresh()
	// but widgets update their own state fine if they are already in the layout.
	// However, adding/removing members needs careful handling.

	// Use a temporary list to track which members we've seen this update
	seen := make(map[string]bool)

	for _, name := range names {
		member, ok := party[name]
		if !ok {
			continue
		}
		seen[name] = true

		widgets, exists := g.memberWidgets[name]
		if !exists {
			widgets = &memberWidgets{
				nameLabel:   widget.NewLabel(name),
				jobLabel:    widget.NewLabel(job.GetJobName(member.Job)),
				hpBar:       widget.NewProgressBar(),
				statusLabel: widget.NewLabel(""),
			}
			widgets.hpBar.Max = 100
			g.memberWidgets[name] = widgets

			row := container.NewHBox(
				container.NewMax(widgets.nameLabel),
				container.NewMax(widgets.jobLabel),
				container.NewGridWrap(fyne.NewSize(200, 20), widgets.hpBar),
				widgets.statusLabel,
			)
			g.partyContainer.Add(row)
		}

		// Update widget values
		widgets.nameLabel.SetText(name)
		widgets.jobLabel.SetText(job.GetJobName(member.Job))
		widgets.hpBar.SetValue(float64(member.HPPercent))

		statusText := ""
		if len(member.StatusIDs) > 0 {
			statusText = fmt.Sprintf("Status: %d", len(member.StatusIDs))
			// If we wanted to be more descriptive, we could look up the most severe status
			if severe := sm.GetMostSevereStatusEffect(member); severe != nil {
				statusText = severe.Name
			}
		}
		widgets.statusLabel.SetText(statusText)
	}

	// Remove members who are no longer in the party
	for name := range g.memberWidgets {
		if !seen[name] {
			// This is a bit tricky with VBox. For simplicity, we might just clear and rebuild
			// if the party changes, but let's try to keep it simple for now.
			// Rebuilding is safer for consistency.
			g.rebuildPartyUI(party, names)
			return
		}
	}
}

func (g *GUI) rebuildPartyUI(party map[string]*statusMonitor.PartyMember, sortedNames []string) {
	g.partyContainer.Objects = nil
	g.memberWidgets = make(map[string]*memberWidgets)

	for _, name := range sortedNames {
		member := party[name]
		widgets := &memberWidgets{
			nameLabel:   widget.NewLabel(name),
			jobLabel:    widget.NewLabel(job.GetJobName(member.Job)),
			hpBar:       widget.NewProgressBar(),
			statusLabel: widget.NewLabel(""),
		}
		widgets.hpBar.Max = 100
		widgets.hpBar.SetValue(float64(member.HPPercent))
		g.memberWidgets[name] = widgets

		row := container.NewHBox(
			container.NewMax(widgets.nameLabel),
			container.NewMax(widgets.jobLabel),
			container.NewGridWrap(fyne.NewSize(200, 20), widgets.hpBar),
			widgets.statusLabel,
		)
		g.partyContainer.Add(row)
	}
	g.partyContainer.Refresh()
}
