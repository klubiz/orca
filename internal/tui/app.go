// Package tui provides a k9s-style Terminal UI for the Orca AI Agent Orchestration system.
package tui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/klubi/orca/pkg/apis/v1alpha1"
	"github.com/klubi/orca/pkg/client"
)

// App is the main TUI application. It polls the Orca REST API and displays
// resources (Pods, Pools, Tasks, Projects) in a navigable table view.
type App struct {
	app        *tview.Application
	pages      *tview.Pages
	header     *tview.TextView
	footer     *tview.TextView
	table      *tview.Table
	filterInput *tview.InputField
	detailView *tview.TextView
	layout     *tview.Flex

	client         *client.Client
	serverAddr     string
	currentView    string // "pods", "pools", "tasks", "projects"
	currentProject string
	filter         string

	// Cached data from the last successful refresh.
	pods     []v1alpha1.AgentPod
	pools    []v1alpha1.AgentPool
	tasks    []v1alpha1.DevTask
	projects []v1alpha1.Project
	lastErr  error

	mu sync.Mutex

	// mainFlex is the outermost vertical flex (header + content + footer).
	mainFlex *tview.Flex

	// describeOpen tracks whether the describe panel is visible.
	describeOpen bool
	// filterOpen tracks whether the filter input is visible.
	filterOpen bool
}

// NewApp creates a new TUI application connected to the given Orca API server.
func NewApp(serverAddr string) *App {
	a := &App{
		app:         tview.NewApplication(),
		client:      client.New(serverAddr),
		serverAddr:  serverAddr,
		currentView: "pods",
	}

	// -- Header --
	a.header = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	a.header.SetBackgroundColor(tcell.ColorDarkBlue)

	// -- Footer --
	a.footer = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	a.footer.SetBackgroundColor(tcell.ColorDarkBlue)

	// -- Table --
	a.table = tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0). // header row stays fixed
		SetSeparator(tview.Borders.Vertical)
	a.table.SetBorder(false)
	a.table.SetBorderPadding(0, 0, 1, 1)

	// -- Filter input --
	a.filterInput = tview.NewInputField().
		SetLabel(" Filter: ").
		SetFieldWidth(40).
		SetFieldBackgroundColor(tcell.ColorBlack).
		SetLabelColor(tcell.ColorYellow)

	a.filterInput.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			a.mu.Lock()
			a.filter = a.filterInput.GetText()
			a.mu.Unlock()
			a.hideFilter()
			a.updateTable()
			a.app.SetFocus(a.table)
		case tcell.KeyEscape:
			a.mu.Lock()
			a.filter = ""
			a.mu.Unlock()
			a.filterInput.SetText("")
			a.hideFilter()
			a.updateTable()
			a.app.SetFocus(a.table)
		}
	})

	// -- Detail / Describe view --
	a.detailView = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true)
	a.detailView.SetBorder(true).
		SetTitle(" Describe ").
		SetBorderColor(tcell.ColorDodgerBlue)

	// -- Build the main layout --
	// contentFlex holds the table (and optionally the detail panel).
	contentFlex := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(a.table, 0, 1, true)

	// mainFlex is the full vertical layout: header, content, footer.
	a.mainFlex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.header, 1, 0, false).
		AddItem(contentFlex, 0, 1, true).
		AddItem(a.footer, 1, 0, false)

	a.layout = contentFlex

	// Pages allows switching between the main view and overlays.
	a.pages = tview.NewPages().
		AddPage("main", a.mainFlex, true, true)

	a.updateHeader()
	a.updateFooter()
	a.setupKeyBindings()

	a.app.SetRoot(a.pages, true).SetFocus(a.table)

	return a
}

// Run starts the background refresh goroutine and runs the TUI event loop.
func (a *App) Run() error {
	// Perform an initial synchronous refresh so the table is populated
	// before the first render.
	a.refresh()

	// Background poller.
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			a.refresh()
			a.app.QueueUpdateDraw(func() {
				a.updateTable()
			})
		}
	}()

	return a.app.Run()
}

// ---------------------------------------------------------------------------
// Key bindings
// ---------------------------------------------------------------------------

func (a *App) setupKeyBindings() {
	a.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// When the filter input has focus, let it handle its own keys.
		if a.filterOpen {
			return event
		}

		// When the describe panel is open, Escape closes it.
		if a.describeOpen && event.Key() == tcell.KeyEscape {
			a.hideDescribe()
			return nil
		}

		switch event.Key() {
		case tcell.KeyRune:
			switch event.Rune() {
			case '1':
				a.switchView("pods")
				return nil
			case '2':
				a.switchView("pools")
				return nil
			case '3':
				a.switchView("tasks")
				return nil
			case '4':
				a.switchView("projects")
				return nil
			case '/':
				a.showFilter()
				return nil
			case 'q':
				a.app.Stop()
				return nil
			case 'r':
				go func() {
					a.refresh()
					a.app.QueueUpdateDraw(func() {
						a.updateTable()
					})
				}()
				return nil
			case 'd':
				a.confirmDelete()
				return nil
			case 'j':
				// Move selection down (vim-style).
				row, _ := a.table.GetSelection()
				if row < a.table.GetRowCount()-1 {
					a.table.Select(row+1, 0)
				}
				return nil
			case 'k':
				// Move selection up (vim-style).
				row, _ := a.table.GetSelection()
				if row > 1 { // row 0 is the header
					a.table.Select(row-1, 0)
				}
				return nil
			}
		case tcell.KeyEnter:
			a.showDescribe()
			return nil
		case tcell.KeyEscape:
			if a.filter != "" {
				a.mu.Lock()
				a.filter = ""
				a.mu.Unlock()
				a.updateTable()
			}
			return nil
		}

		return event
	})
}

// ---------------------------------------------------------------------------
// View switching
// ---------------------------------------------------------------------------

func (a *App) switchView(view string) {
	a.mu.Lock()
	a.currentView = view
	a.mu.Unlock()

	a.updateHeader()

	go func() {
		a.refresh()
		a.app.QueueUpdateDraw(func() {
			a.updateTable()
		})
	}()
}

// ---------------------------------------------------------------------------
// Data refresh
// ---------------------------------------------------------------------------

func (a *App) refresh() {
	a.mu.Lock()
	view := a.currentView
	project := a.currentProject
	a.mu.Unlock()

	switch view {
	case "pods":
		pods, err := a.client.ListAgentPods(project)
		a.mu.Lock()
		a.pods = pods
		a.lastErr = err
		a.mu.Unlock()
	case "pools":
		pools, err := a.client.ListAgentPools(project)
		a.mu.Lock()
		a.pools = pools
		a.lastErr = err
		a.mu.Unlock()
	case "tasks":
		tasks, err := a.client.ListDevTasks(project)
		a.mu.Lock()
		a.tasks = tasks
		a.lastErr = err
		a.mu.Unlock()
	case "projects":
		projects, err := a.client.ListProjects()
		a.mu.Lock()
		a.projects = projects
		a.lastErr = err
		a.mu.Unlock()
	}
}

// ---------------------------------------------------------------------------
// Table rendering
// ---------------------------------------------------------------------------

func (a *App) updateTable() {
	a.table.Clear()

	a.mu.Lock()
	view := a.currentView
	filter := strings.ToLower(a.filter)
	err := a.lastErr
	a.mu.Unlock()

	if err != nil {
		a.setTableHeaders([]string{"ERROR"})
		a.table.SetCell(1, 0,
			tview.NewTableCell(fmt.Sprintf("Error: %v", err)).
				SetTextColor(tcell.ColorRed))
		return
	}

	switch view {
	case "pods":
		a.renderPods(filter)
	case "pools":
		a.renderPools(filter)
	case "tasks":
		a.renderTasks(filter)
	case "projects":
		a.renderProjects(filter)
	}

	// Ensure a row is selected.
	if a.table.GetRowCount() > 1 {
		a.table.Select(1, 0)
	}
}

func (a *App) setTableHeaders(headers []string) {
	for col, h := range headers {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorWhite).
			SetBackgroundColor(tcell.ColorDarkCyan).
			SetAttributes(tcell.AttrBold).
			SetSelectable(false).
			SetExpansion(1)
		a.table.SetCell(0, col, cell)
	}
}

// matchesFilter returns true if any of the values contain the filter string.
func matchesFilter(filter string, values ...string) bool {
	if filter == "" {
		return true
	}
	for _, v := range values {
		if strings.Contains(strings.ToLower(v), filter) {
			return true
		}
	}
	return false
}

func (a *App) renderPods(filter string) {
	headers := []string{"NAME", "PROJECT", "MODEL", "PHASE", "ACTIVE-TASKS", "AGE"}
	a.setTableHeaders(headers)

	a.mu.Lock()
	pods := a.pods
	a.mu.Unlock()

	row := 1
	for _, p := range pods {
		phase := string(p.Status.Phase)
		active := fmt.Sprintf("%d", p.Status.ActiveTasks)
		age := formatAge(p.Metadata.CreatedAt)

		if !matchesFilter(filter, p.Metadata.Name, p.Metadata.Project, p.Spec.Model, phase, active, age) {
			continue
		}

		a.table.SetCell(row, 0, tview.NewTableCell(p.Metadata.Name).SetExpansion(1))
		a.table.SetCell(row, 1, tview.NewTableCell(p.Metadata.Project).SetExpansion(1))
		a.table.SetCell(row, 2, tview.NewTableCell(p.Spec.Model).SetExpansion(1))
		a.table.SetCell(row, 3, tview.NewTableCell(phase).
			SetTextColor(phaseColor(phase)).SetExpansion(1))
		a.table.SetCell(row, 4, tview.NewTableCell(active).SetExpansion(1))
		a.table.SetCell(row, 5, tview.NewTableCell(age).SetExpansion(1))
		row++
	}
}

func (a *App) renderPools(filter string) {
	headers := []string{"NAME", "PROJECT", "REPLICAS", "READY", "BUSY", "AGE"}
	a.setTableHeaders(headers)

	a.mu.Lock()
	pools := a.pools
	a.mu.Unlock()

	row := 1
	for _, p := range pools {
		replicas := fmt.Sprintf("%d", p.Spec.Replicas)
		ready := fmt.Sprintf("%d", p.Status.ReadyReplicas)
		busy := fmt.Sprintf("%d", p.Status.BusyReplicas)
		age := formatAge(p.Metadata.CreatedAt)

		if !matchesFilter(filter, p.Metadata.Name, p.Metadata.Project, replicas, ready, busy, age) {
			continue
		}

		a.table.SetCell(row, 0, tview.NewTableCell(p.Metadata.Name).SetExpansion(1))
		a.table.SetCell(row, 1, tview.NewTableCell(p.Metadata.Project).SetExpansion(1))
		a.table.SetCell(row, 2, tview.NewTableCell(replicas).SetExpansion(1))
		a.table.SetCell(row, 3, tview.NewTableCell(ready).
			SetTextColor(tcell.ColorGreen).SetExpansion(1))
		a.table.SetCell(row, 4, tview.NewTableCell(busy).
			SetTextColor(tcell.ColorYellow).SetExpansion(1))
		a.table.SetCell(row, 5, tview.NewTableCell(age).SetExpansion(1))
		row++
	}
}

func (a *App) renderTasks(filter string) {
	headers := []string{"NAME", "PROJECT", "PHASE", "ASSIGNED-POD", "RETRIES", "AGE"}
	a.setTableHeaders(headers)

	a.mu.Lock()
	tasks := a.tasks
	a.mu.Unlock()

	row := 1
	for _, t := range tasks {
		phase := string(t.Status.Phase)
		retries := fmt.Sprintf("%d", t.Status.Retries)
		age := formatAge(t.Metadata.CreatedAt)

		if !matchesFilter(filter, t.Metadata.Name, t.Metadata.Project, phase, t.Status.AssignedPod, retries, age) {
			continue
		}

		a.table.SetCell(row, 0, tview.NewTableCell(t.Metadata.Name).SetExpansion(1))
		a.table.SetCell(row, 1, tview.NewTableCell(t.Metadata.Project).SetExpansion(1))
		a.table.SetCell(row, 2, tview.NewTableCell(phase).
			SetTextColor(phaseColor(phase)).SetExpansion(1))
		a.table.SetCell(row, 3, tview.NewTableCell(t.Status.AssignedPod).SetExpansion(1))
		a.table.SetCell(row, 4, tview.NewTableCell(retries).SetExpansion(1))
		a.table.SetCell(row, 5, tview.NewTableCell(age).SetExpansion(1))
		row++
	}
}

func (a *App) renderProjects(filter string) {
	headers := []string{"NAME", "DESCRIPTION", "AGE"}
	a.setTableHeaders(headers)

	a.mu.Lock()
	projects := a.projects
	a.mu.Unlock()

	row := 1
	for _, p := range projects {
		age := formatAge(p.Metadata.CreatedAt)

		if !matchesFilter(filter, p.Metadata.Name, p.Spec.Description, age) {
			continue
		}

		a.table.SetCell(row, 0, tview.NewTableCell(p.Metadata.Name).SetExpansion(1))
		a.table.SetCell(row, 1, tview.NewTableCell(p.Spec.Description).SetExpansion(1))
		a.table.SetCell(row, 2, tview.NewTableCell(age).SetExpansion(1))
		row++
	}
}

// ---------------------------------------------------------------------------
// Describe (detail panel)
// ---------------------------------------------------------------------------

func (a *App) showDescribe() {
	row, _ := a.table.GetSelection()
	if row < 1 || row >= a.table.GetRowCount() {
		return
	}

	name := a.table.GetCell(row, 0).Text
	project := ""
	// For non-project views, column 1 is the project.
	if a.currentView != "projects" && a.table.GetColumnCount() > 1 {
		project = a.table.GetCell(row, 1).Text
	}

	a.detailView.Clear()

	a.mu.Lock()
	view := a.currentView
	a.mu.Unlock()

	var detail string

	switch view {
	case "pods":
		pod, err := a.client.GetAgentPod(name, project)
		if err != nil {
			detail = fmt.Sprintf("[red]Error: %v[-]", err)
		} else {
			detail = a.formatPodDescribe(pod)
		}
	case "pools":
		pool, err := a.client.GetAgentPool(name, project)
		if err != nil {
			detail = fmt.Sprintf("[red]Error: %v[-]", err)
		} else {
			detail = a.formatPoolDescribe(pool)
		}
	case "tasks":
		task, err := a.client.GetDevTask(name, project)
		if err != nil {
			detail = fmt.Sprintf("[red]Error: %v[-]", err)
		} else {
			detail = a.formatTaskDescribe(task)
		}
	case "projects":
		proj, err := a.client.GetProject(name)
		if err != nil {
			detail = fmt.Sprintf("[red]Error: %v[-]", err)
		} else {
			detail = a.formatProjectDescribe(proj)
		}
	}

	a.detailView.SetText(detail)

	if !a.describeOpen {
		a.layout.AddItem(a.detailView, 0, 1, false)
		a.describeOpen = true
	}
}

func (a *App) hideDescribe() {
	if a.describeOpen {
		a.layout.RemoveItem(a.detailView)
		a.describeOpen = false
		a.app.SetFocus(a.table)
	}
}

func (a *App) formatPodDescribe(pod *v1alpha1.AgentPod) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("[::b]Name:[-::-]          %s\n", pod.Metadata.Name))
	b.WriteString(fmt.Sprintf("[::b]Project:[-::-]       %s\n", pod.Metadata.Project))
	b.WriteString(fmt.Sprintf("[::b]UID:[-::-]           %s\n", pod.Metadata.UID))
	b.WriteString(fmt.Sprintf("[::b]Model:[-::-]         %s\n", pod.Spec.Model))
	b.WriteString(fmt.Sprintf("[::b]Phase:[-::-]         [%s]%s[-]\n",
		phaseColorName(string(pod.Status.Phase)), pod.Status.Phase))
	b.WriteString(fmt.Sprintf("[::b]Active Tasks:[-::-]  %d\n", pod.Status.ActiveTasks))
	b.WriteString(fmt.Sprintf("[::b]Completed:[-::-]     %d\n", pod.Status.CompletedTasks))
	b.WriteString(fmt.Sprintf("[::b]Failed:[-::-]        %d\n", pod.Status.FailedTasks))
	b.WriteString(fmt.Sprintf("[::b]Max Concurrency:[-::-] %d\n", pod.Spec.MaxConcurrency))
	b.WriteString(fmt.Sprintf("[::b]Max Tokens:[-::-]    %d\n", pod.Spec.MaxTokens))
	b.WriteString(fmt.Sprintf("[::b]Restart Policy:[-::-] %s\n", pod.Spec.RestartPolicy))
	b.WriteString(fmt.Sprintf("[::b]Owner Pool:[-::-]    %s\n", pod.Spec.OwnerPool))
	if pod.Status.Message != "" {
		b.WriteString(fmt.Sprintf("[::b]Message:[-::-]       %s\n", pod.Status.Message))
	}
	b.WriteString(fmt.Sprintf("[::b]Created:[-::-]       %s\n", pod.Metadata.CreatedAt.Format(time.RFC3339)))
	if !pod.Status.StartedAt.IsZero() {
		b.WriteString(fmt.Sprintf("[::b]Started:[-::-]       %s\n", pod.Status.StartedAt.Format(time.RFC3339)))
	}
	if !pod.Status.LastHeartbeat.IsZero() {
		b.WriteString(fmt.Sprintf("[::b]Last Heartbeat:[-::-] %s\n", pod.Status.LastHeartbeat.Format(time.RFC3339)))
	}

	if len(pod.Spec.Capabilities) > 0 {
		b.WriteString(fmt.Sprintf("[::b]Capabilities:[-::-]  %s\n", strings.Join(pod.Spec.Capabilities, ", ")))
	}
	if len(pod.Spec.Tools) > 0 {
		b.WriteString(fmt.Sprintf("[::b]Tools:[-::-]         %s\n", strings.Join(pod.Spec.Tools, ", ")))
	}

	if len(pod.Metadata.Labels) > 0 {
		b.WriteString("[::b]Labels:[-::-]\n")
		for k, v := range pod.Metadata.Labels {
			b.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}

	return b.String()
}

func (a *App) formatPoolDescribe(pool *v1alpha1.AgentPool) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("[::b]Name:[-::-]           %s\n", pool.Metadata.Name))
	b.WriteString(fmt.Sprintf("[::b]Project:[-::-]        %s\n", pool.Metadata.Project))
	b.WriteString(fmt.Sprintf("[::b]UID:[-::-]            %s\n", pool.Metadata.UID))
	b.WriteString(fmt.Sprintf("[::b]Replicas:[-::-]       %d (desired) / %d (current)\n",
		pool.Spec.Replicas, pool.Status.Replicas))
	b.WriteString(fmt.Sprintf("[::b]Ready:[-::-]          [green]%d[-]\n", pool.Status.ReadyReplicas))
	b.WriteString(fmt.Sprintf("[::b]Busy:[-::-]           [yellow]%d[-]\n", pool.Status.BusyReplicas))
	b.WriteString(fmt.Sprintf("[::b]Template Model:[-::-] %s\n", pool.Spec.Template.Spec.Model))
	b.WriteString(fmt.Sprintf("[::b]Created:[-::-]        %s\n", pool.Metadata.CreatedAt.Format(time.RFC3339)))

	if len(pool.Spec.Selector) > 0 {
		b.WriteString("[::b]Selector:[-::-]\n")
		for k, v := range pool.Spec.Selector {
			b.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}
	if len(pool.Metadata.Labels) > 0 {
		b.WriteString("[::b]Labels:[-::-]\n")
		for k, v := range pool.Metadata.Labels {
			b.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}

	return b.String()
}

func (a *App) formatTaskDescribe(task *v1alpha1.DevTask) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("[::b]Name:[-::-]         %s\n", task.Metadata.Name))
	b.WriteString(fmt.Sprintf("[::b]Project:[-::-]      %s\n", task.Metadata.Project))
	b.WriteString(fmt.Sprintf("[::b]UID:[-::-]          %s\n", task.Metadata.UID))
	b.WriteString(fmt.Sprintf("[::b]Phase:[-::-]        [%s]%s[-]\n",
		phaseColorName(string(task.Status.Phase)), task.Status.Phase))
	b.WriteString(fmt.Sprintf("[::b]Assigned Pod:[-::-] %s\n", task.Status.AssignedPod))
	b.WriteString(fmt.Sprintf("[::b]Retries:[-::-]      %d / %d\n",
		task.Status.Retries, task.Spec.MaxRetries))
	b.WriteString(fmt.Sprintf("[::b]Prompt:[-::-]\n  %s\n", task.Spec.Prompt))

	if task.Spec.PreferredModel != "" {
		b.WriteString(fmt.Sprintf("[::b]Preferred Model:[-::-] %s\n", task.Spec.PreferredModel))
	}
	if task.Spec.TimeoutSeconds > 0 {
		b.WriteString(fmt.Sprintf("[::b]Timeout:[-::-]      %ds\n", task.Spec.TimeoutSeconds))
	}
	if len(task.Spec.RequiredCapabilities) > 0 {
		b.WriteString(fmt.Sprintf("[::b]Required Caps:[-::-] %s\n",
			strings.Join(task.Spec.RequiredCapabilities, ", ")))
	}
	if len(task.Spec.DependsOn) > 0 {
		b.WriteString(fmt.Sprintf("[::b]Depends On:[-::-]   %s\n",
			strings.Join(task.Spec.DependsOn, ", ")))
	}

	b.WriteString(fmt.Sprintf("[::b]Created:[-::-]      %s\n", task.Metadata.CreatedAt.Format(time.RFC3339)))
	if !task.Status.StartedAt.IsZero() {
		b.WriteString(fmt.Sprintf("[::b]Started:[-::-]      %s\n", task.Status.StartedAt.Format(time.RFC3339)))
	}
	if !task.Status.FinishedAt.IsZero() {
		b.WriteString(fmt.Sprintf("[::b]Finished:[-::-]     %s\n", task.Status.FinishedAt.Format(time.RFC3339)))
	}

	if task.Status.Output != "" {
		b.WriteString(fmt.Sprintf("\n[::b]Output:[-::-]\n%s\n", task.Status.Output))
	}
	if task.Status.Error != "" {
		b.WriteString(fmt.Sprintf("\n[red][::b]Error:[-::-]\n%s[-]\n", task.Status.Error))
	}

	if len(task.Metadata.Labels) > 0 {
		b.WriteString("[::b]Labels:[-::-]\n")
		for k, v := range task.Metadata.Labels {
			b.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}

	return b.String()
}

func (a *App) formatProjectDescribe(proj *v1alpha1.Project) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("[::b]Name:[-::-]        %s\n", proj.Metadata.Name))
	b.WriteString(fmt.Sprintf("[::b]UID:[-::-]         %s\n", proj.Metadata.UID))
	b.WriteString(fmt.Sprintf("[::b]Description:[-::-] %s\n", proj.Spec.Description))
	b.WriteString(fmt.Sprintf("[::b]Path:[-::-]        %s\n", proj.Spec.Path))
	b.WriteString(fmt.Sprintf("[::b]Status:[-::-]      %s\n", proj.Status))
	b.WriteString(fmt.Sprintf("[::b]Created:[-::-]     %s\n", proj.Metadata.CreatedAt.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("[::b]Updated:[-::-]     %s\n", proj.Metadata.UpdatedAt.Format(time.RFC3339)))

	if len(proj.Metadata.Labels) > 0 {
		b.WriteString("[::b]Labels:[-::-]\n")
		for k, v := range proj.Metadata.Labels {
			b.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// Filter
// ---------------------------------------------------------------------------

func (a *App) showFilter() {
	if a.filterOpen {
		return
	}
	a.filterOpen = true
	a.filterInput.SetText(a.filter)

	// Replace footer with filter input in the main vertical flex.
	a.mainFlex.RemoveItem(a.footer)
	a.mainFlex.AddItem(a.filterInput, 1, 0, true)
	a.app.SetFocus(a.filterInput)
}

func (a *App) hideFilter() {
	if !a.filterOpen {
		return
	}
	a.filterOpen = false

	// Restore footer in place of filter input.
	a.mainFlex.RemoveItem(a.filterInput)
	a.mainFlex.AddItem(a.footer, 1, 0, false)
	a.app.SetFocus(a.table)
}

// ---------------------------------------------------------------------------
// Delete with confirmation
// ---------------------------------------------------------------------------

func (a *App) confirmDelete() {
	row, _ := a.table.GetSelection()
	if row < 1 || row >= a.table.GetRowCount() {
		return
	}

	name := a.table.GetCell(row, 0).Text
	project := ""
	if a.currentView != "projects" && a.table.GetColumnCount() > 1 {
		project = a.table.GetCell(row, 1).Text
	}

	modal := tview.NewModal().
		SetText(fmt.Sprintf("Delete %s \"%s\"?", a.currentView[:len(a.currentView)-1], name)).
		AddButtons([]string{"Delete", "Cancel"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonLabel == "Delete" {
				a.deleteResource(name, project)
			}
			a.pages.RemovePage("confirm")
			a.app.SetFocus(a.table)
		})
	modal.SetBackgroundColor(tcell.ColorDarkRed)

	a.pages.AddPage("confirm", modal, true, true)
}

func (a *App) deleteResource(name, project string) {
	a.mu.Lock()
	view := a.currentView
	a.mu.Unlock()

	var err error
	switch view {
	case "pods":
		err = a.client.DeleteAgentPod(name, project)
	case "pools":
		err = a.client.DeleteAgentPool(name, project)
	case "tasks":
		err = a.client.DeleteDevTask(name, project)
	case "projects":
		err = a.client.DeleteProject(name)
	}

	if err != nil {
		// Show error briefly in footer or just log it.
		a.footer.SetText(fmt.Sprintf(" [red]Delete failed: %v[-]", err))
		go func() {
			time.Sleep(3 * time.Second)
			a.app.QueueUpdateDraw(func() {
				a.updateFooter()
			})
		}()
		return
	}

	// Refresh immediately after delete.
	go func() {
		a.refresh()
		a.app.QueueUpdateDraw(func() {
			a.updateTable()
		})
	}()
}

// ---------------------------------------------------------------------------
// Header & Footer
// ---------------------------------------------------------------------------

func (a *App) updateHeader() {
	views := []struct {
		key  string
		name string
	}{
		{"1", "Pods"},
		{"2", "Pools"},
		{"3", "Tasks"},
		{"4", "Projects"},
	}

	viewMap := map[string]string{
		"1": "pods",
		"2": "pools",
		"3": "tasks",
		"4": "projects",
	}

	var parts []string
	for _, v := range views {
		if viewMap[v.key] == a.currentView {
			parts = append(parts, fmt.Sprintf("[::b]<%s>[%s][::-]", v.key, v.name))
		} else {
			parts = append(parts, fmt.Sprintf("<%s>%s", v.key, v.name))
		}
	}

	filterInfo := ""
	a.mu.Lock()
	if a.filter != "" {
		filterInfo = fmt.Sprintf(" | [yellow]filter: %s[-]", a.filter)
	}
	a.mu.Unlock()

	a.header.SetText(fmt.Sprintf(" [::b]Orca[::-] | %s | %s%s",
		a.serverAddr, strings.Join(parts, "  "), filterInfo))
}

func (a *App) updateFooter() {
	a.footer.SetText(" [yellow]<enter>[white]Describe  [yellow]<d>[white]Delete  [yellow]</>[white]Filter  [yellow]<q>[white]Quit  [yellow]<r>[white]Refresh  [yellow]<esc>[white]Back")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// formatAge returns a human-readable duration string since the given time.
func formatAge(t time.Time) string {
	if t.IsZero() {
		return "-"
	}

	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// phaseColor returns the tcell color appropriate for a phase string.
func phaseColor(phase string) tcell.Color {
	switch phase {
	case "Ready", "Succeeded":
		return tcell.ColorGreen
	case "Running", "Busy", "Starting":
		return tcell.ColorYellow
	case "Pending", "Scheduled":
		return tcell.ColorWhite
	case "Failed":
		return tcell.ColorRed
	case "Terminating", "Terminated":
		return tcell.ColorGray
	default:
		return tcell.ColorWhite
	}
}

// phaseColorName returns the tview color tag name for a phase string.
func phaseColorName(phase string) string {
	switch phase {
	case "Ready", "Succeeded":
		return "green"
	case "Running", "Busy", "Starting":
		return "yellow"
	case "Pending", "Scheduled":
		return "white"
	case "Failed":
		return "red"
	case "Terminating", "Terminated":
		return "gray"
	default:
		return "white"
	}
}
