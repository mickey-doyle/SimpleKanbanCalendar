package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	ical "github.com/arran4/golang-ical"
)

// --- Constants & Data ---

type ItemType string

const (
	TypeTask  ItemType = "Task"
	TypeEvent ItemType = "Event"
)

var PresetColors = []string{
	"#E74C3C", "#E67E22", "#F1C40F", "#2ECC71",
	"#1ABC9C", "#3498DB", "#9B59B6", "#34495E",
	"#7F8C8D", "#D35400", "#27AE60", "#8E44AD",
}

type Group struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ColorHex string `json:"color"`
	SortMode string `json:"sortMode"`
}

type TodoItem struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Start     string   `json:"start"`
	End       string   `json:"end"`
	Type      ItemType `json:"type"`
	GroupID   string   `json:"groupId"`
	GroupName string   `json:"group,omitempty"`
	Completed bool     `json:"completed"`
	SeriesID  string   `json:"seriesId,omitempty"`
}

// Global Data
var items []TodoItem
var groups []Group
var availableCalendars []string
var activeCalendarName string = "Default"
var currentViewDate time.Time
var selectedCalendarDate time.Time
var currentTheme string = "Dark"

// UI Globals
var myApp fyne.App
var mainWindow fyne.Window
var calendarGrid *fyne.Container
var kanbanContainer *fyne.Container
var monthLabel *widget.Label

// Sidebar Globals
var sbTitleEntry *widget.Entry
var sbGroupSelect *widget.Select
var sbTypeSelect *widget.Select
var sbActionBtn *widget.Button
var sbCancelBtn *widget.Button
var sbDeleteBtn *widget.Button
var sbHeaderLabel *widget.Label
var currentEditItemID string

// Recurrence Globals
var recCheck *widget.Check
var recContainer *fyne.Container
var recModeRadio *widget.RadioGroup
var recNumEntry *widget.Entry
var recUnitSelect *widget.Select
var recOrdinalSelect *widget.Select
var recDaySelect *widget.Select

// Date/Time Setters
var setTaskDate func(string)
var setTaskTime func(string, string, string)
var setStartDate func(string)
var setStartTime func(string, string, string)
var setEndDate func(string)
var setEndTime func(string, string, string)

// Auto-Save Getters
var getTaskDateVal func() string
var getTaskTimeVal func() (string, string, string)
var getStartDateVal func() string
var getStartTimeVal func() (string, string, string)
var getEndDateVal func() string
var getEndTimeVal func() (string, string, string)

// --- Main Entry ---

func main() {
	myApp = app.New()
	myApp.Settings().SetTheme(theme.DarkTheme())
	currentTheme = "Dark"

	mainWindow = myApp.NewWindow("Go Local Calendar & Kanban")
	mainWindow.Resize(fyne.NewSize(1300, 850))

	loadCalendarList()
	loadGroups()
	loadData()
	currentViewDate = time.Now()
	selectedCalendarDate = time.Now()

	settingsBtn := widget.NewButtonWithIcon("", theme.SettingsIcon(), func() {
		showSettingsDialog()
	})
	topBar := container.NewHBox(layout.NewSpacer(), settingsBtn)

	sidebar := createSidebar()
	calendarView := createCalendarArea()
	kanbanView := createKanbanArea()

	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("Calendar", theme.ContentPasteIcon(), calendarView),
		container.NewTabItemWithIcon("Kanban Board", theme.GridIcon(), kanbanView),
	)

	tabs.OnSelected = func(ti *container.TabItem) {
		refreshCalendar()
		refreshKanban()
	}

	split := container.NewHSplit(sidebar, tabs)
	split.SetOffset(0.35)

	content := container.NewBorder(topBar, nil, nil, nil, split)

	mainWindow.SetContent(content)
	mainWindow.ShowAndRun()
}

// --- CUSTOM WIDGETS (DEFINED HERE TO PREVENT ERRORS) ---

type clickableBox struct {
	widget.BaseWidget
	content *fyne.Container
	onTap   func()
	onRight func(*fyne.PointEvent)
}

func newClickableBox(c *fyne.Container, fn func()) *clickableBox {
	b := &clickableBox{content: c, onTap: fn}
	b.ExtendBaseWidget(b)
	return b
}

func (b *clickableBox) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(b.content)
}

func (b *clickableBox) Tapped(_ *fyne.PointEvent) {
	if b.onTap != nil {
		b.onTap()
	}
}

func (b *clickableBox) TappedSecondary(e *fyne.PointEvent) {
	if b.onRight != nil {
		b.onRight(e)
	}
}

func createStrikethroughText(text string, col color.Color, textSize float32) *fyne.Container {
	txt := canvas.NewText(text, col)
	txt.TextSize = textSize
	size := txt.MinSize()
	line := canvas.NewRectangle(col)
	line.SetMinSize(fyne.NewSize(size.Width, 2))
	lineContainer := container.NewVBox(layout.NewSpacer(), line, layout.NewSpacer())
	return container.NewStack(txt, lineContainer)
}

// --- AUTO SAVE LOGIC ---

func autoSave() {
	if currentEditItemID == "" {
		return
	}
	if sbTitleEntry.Text == "" {
		return
	}

	var targetItem *TodoItem
	for i := range items {
		if items[i].ID == currentEditItemID {
			targetItem = &items[i]
			break
		}
	}
	if targetItem == nil {
		return
	}

	targetItem.Title = sbTitleEntry.Text

	for _, g := range groups {
		if g.Name == sbGroupSelect.Selected {
			targetItem.GroupID = g.ID
			break
		}
	}

	targetItem.Type = TypeTask
	if sbTypeSelect.Selected == "Event" {
		targetItem.Type = TypeEvent
	}

	combine := func(dateStr, h, m, ap string) string {
		hour, _ := strconv.Atoi(h)
		if ap == "PM" && hour != 12 {
			hour += 12
		}
		if ap == "AM" && hour == 12 {
			hour = 0
		}
		return fmt.Sprintf("%s %02d:%s", dateStr, hour, m)
	}

	if targetItem.Type == TypeTask {
		if getTaskTimeVal != nil && getTaskDateVal != nil {
			h, m, ap := getTaskTimeVal()
			targetItem.Start = combine(getTaskDateVal(), h, m, ap)
			targetItem.End = targetItem.Start
		}
	} else {
		if getStartTimeVal != nil && getStartDateVal != nil && getEndTimeVal != nil && getEndDateVal != nil {
			hS, mS, apS := getStartTimeVal()
			targetItem.Start = combine(getStartDateVal(), hS, mS, apS)
			hE, mE, apE := getEndTimeVal()
			targetItem.End = combine(getEndDateVal(), hE, mE, apE)
		}
	}

	saveData()
	refreshCalendar()
	refreshKanban()
}

// --- SIDEBAR UI ---

func createSidebar() fyne.CanvasObject {
	sbHeaderLabel = widget.NewLabelWithStyle("Add New Item", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	sbTitleEntry = widget.NewEntry()
	sbTitleEntry.PlaceHolder = "Title"
	sbTitleEntry.OnChanged = func(s string) { autoSave() }

	sbTypeSelect = widget.NewSelect([]string{"Task", "Event"}, nil)
	sbTypeSelect.PlaceHolder = "Select Type"

	sbGroupSelect = widget.NewSelect([]string{}, nil)
	sbGroupSelect.PlaceHolder = "Select Group"
	updateGroupDropdown()
	if len(groups) > 0 {
		sbGroupSelect.SetSelected(groups[0].Name)
	}

	sbGroupSelect.OnChanged = func(s string) {
		if s == "+ Create New Group" {
			showGroupForm(nil)
			sbGroupSelect.SetSelected("")
		} else {
			autoSave()
		}
	}

	btnManageGroups := widget.NewButton("Manage Groups", func() { showGroupManager() })

	// Task Inputs
	lblDeadline := widget.NewLabel("Deadline")
	btnDateDead, getDeadDate, setDeadDate := createDatePickerButton(mainWindow, func(s string) { autoSave() })
	setTaskDate = setDeadDate
	getTaskDateVal = getDeadDate

	hDead, mDead, apDead, contTimeDead, setTTime := createTimePicker(func() { autoSave() })
	setTaskTime = setTTime
	getTaskTimeVal = func() (string, string, string) { return hDead.Selected, mDead.Selected, apDead.Selected }

	taskContainer := container.NewVBox(lblDeadline, container.NewGridWithColumns(2, btnDateDead, contTimeDead))

	// Event Inputs
	lblStart := widget.NewLabel("Start Time")
	var getEndDateStr func() string
	var setEndDateStr func(string)

	onStartChange := func(newStart string) {
		if getEndDateStr != nil && setEndDateStr != nil {
			currentEnd := getEndDateStr()
			sTime, err1 := time.Parse("2006-01-02", newStart)
			eTime, err2 := time.Parse("2006-01-02", currentEnd)
			if err1 == nil && err2 == nil && sTime.After(eTime) {
				setEndDateStr(newStart)
			}
		}
		autoSave()
	}

	btnDateStart, getStartDate, setSDate := createDatePickerButton(mainWindow, onStartChange)
	setStartDate = setSDate
	getStartDateVal = getStartDate

	hStart, mStart, apStart, contTimeStart, setSTime := createTimePicker(func() { autoSave() })
	setStartTime = setSTime
	getStartTimeVal = func() (string, string, string) { return hStart.Selected, mStart.Selected, apStart.Selected }

	lblEnd := widget.NewLabel("End Time")
	btnDateEnd, getEnd, setE := createDatePickerButton(mainWindow, func(s string) { autoSave() })
	getEndDateStr = getEnd
	setEndDateStr = setE
	setEndDate = setE
	getEndDateVal = getEnd

	hEnd, mEnd, apEnd, contTimeEnd, setETime := createTimePicker(func() { autoSave() })
	setEndTime = setETime
	getEndTimeVal = func() (string, string, string) { return hEnd.Selected, mEnd.Selected, apEnd.Selected }

	eventContainer := container.NewVBox(lblStart, container.NewGridWithColumns(2, btnDateStart, contTimeStart), lblEnd, container.NewGridWithColumns(2, btnDateEnd, contTimeEnd))

	dynamicArea := container.NewVBox()

	sbTypeSelect.OnChanged = func(s string) {
		dynamicArea.Objects = nil
		if s == "Task" {
			sbTitleEntry.PlaceHolder = "Task Title"
			dynamicArea.Add(taskContainer)
		} else {
			sbTitleEntry.PlaceHolder = "Event Title"
			dynamicArea.Add(eventContainer)
		}
		dynamicArea.Refresh()
		updateSidebarHeader()
		autoSave()
	}

	recCheck = widget.NewCheck("Recurring?", func(b bool) {
		if b {
			recContainer.Show()
		} else {
			recContainer.Hide()
		}
	})
	recNumEntry = widget.NewEntry()
	recNumEntry.SetText("1")
	recUnitSelect = widget.NewSelect([]string{"Day(s)", "Week(s)", "Month(s)", "Year(s)"}, nil)
	recUnitSelect.SetSelected("Week(s)")
	method1Content := container.NewGridWithColumns(2, container.NewBorder(nil, nil, widget.NewLabel("Every"), nil, recNumEntry), recUnitSelect)
	recOrdinalSelect = widget.NewSelect([]string{"Every", "Every Other"}, nil)
	recOrdinalSelect.SetSelected("Every")
	recDaySelect = widget.NewSelect([]string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}, nil)
	recDaySelect.SetSelected("Monday")
	method2Content := container.NewGridWithColumns(2, recOrdinalSelect, recDaySelect)
	recModeRadio = widget.NewRadioGroup([]string{"Interval", "Specific Day"}, func(s string) {
		if s == "Interval" {
			recNumEntry.Enable()
			recUnitSelect.Enable()
			recOrdinalSelect.Disable()
			recDaySelect.Disable()
		} else {
			recNumEntry.Disable()
			recUnitSelect.Disable()
			recOrdinalSelect.Enable()
			recDaySelect.Enable()
		}
	})
	recModeRadio.SetSelected("Interval")
	recContainer = container.NewVBox(recModeRadio, method1Content, method2Content)
	recContainer.Hide()

	sbActionBtn = widget.NewButtonWithIcon("Add Item", theme.ContentAddIcon(), func() {
		handleSidebarAction()
	})
	sbActionBtn.Importance = widget.HighImportance

	sbCancelBtn = widget.NewButton("Done Editing", func() { resetSidebar() })
	sbCancelBtn.Hide()

	sbDeleteBtn = widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		if currentEditItemID == "" {
			return
		}
		performSmartDelete(currentEditItemID)
	})
	sbDeleteBtn.Hide()

	exportBtn := widget.NewButton("Export .ICS", exportICS)

	topPart := container.NewVBox(
		sbHeaderLabel,
		widget.NewLabel("Type"), sbTypeSelect,
		widget.NewLabel("Title"), sbTitleEntry,
		widget.NewLabel("Group"), sbGroupSelect,
		btnManageGroups,
	)

	bottomPart := container.NewVBox(
		recCheck, recContainer,
		layout.NewSpacer(),
		container.NewHBox(sbActionBtn, sbDeleteBtn),
		sbCancelBtn,
		exportBtn,
	)

	sbTypeSelect.SetSelected("Task")

	return container.NewPadded(container.NewBorder(topPart, bottomPart, nil, nil, dynamicArea))
}

// --- LOGIC: ADD ---

func handleSidebarAction() {
	if sbTitleEntry.Text == "" {
		dialog.ShowError(fmt.Errorf("title required"), mainWindow)
		return
	}
	var selectedGroupID string
	for _, g := range groups {
		if g.Name == sbGroupSelect.Selected {
			selectedGroupID = g.ID
			break
		}
	}
	if selectedGroupID == "" && len(groups) > 0 {
		selectedGroupID = groups[0].ID
		sbGroupSelect.SetSelected(groups[0].Name)
	}
	if selectedGroupID == "" {
		dialog.ShowError(fmt.Errorf("please select a group"), mainWindow)
		return
	}

	combine := func(dateStr, h, m, ap string) string {
		hour, _ := strconv.Atoi(h)
		if ap == "PM" && hour != 12 {
			hour += 12
		}
		if ap == "AM" && hour == 12 {
			hour = 0
		}
		return fmt.Sprintf("%s %02d:%s", dateStr, hour, m)
	}

	var sVal, eVal string
	curType := TypeTask
	if sbTypeSelect.Selected == "Event" {
		curType = TypeEvent
		hS, mS, apS := getStartTimeVal()
		sVal = combine(getStartDateVal(), hS, mS, apS)
		hE, mE, apE := getEndTimeVal()
		eVal = combine(getEndDateVal(), hE, mE, apE)
	} else {
		hD, mD, apD := getTaskTimeVal()
		sVal = combine(getTaskDateVal(), hD, mD, apD)
		eVal = sVal
	}

	newSeriesID := ""
	if recCheck.Checked {
		newSeriesID = fmt.Sprintf("s-%d", time.Now().UnixNano())
	}

	baseStart, _ := time.ParseInLocation("2006-01-02 15:04", sVal, time.Local)
	baseEnd, _ := time.ParseInLocation("2006-01-02 15:04", eVal, time.Local)
	duration := baseEnd.Sub(baseStart)

	itemsToCreate := []TodoItem{}
	baseItem := TodoItem{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Title:     sbTitleEntry.Text,
		GroupID:   selectedGroupID,
		Type:      curType,
		Start:     sVal,
		End:       eVal,
		SeriesID:  newSeriesID,
		Completed: false,
	}
	itemsToCreate = append(itemsToCreate, baseItem)

	if recCheck.Checked {
		limitDate := baseStart.AddDate(1, 0, 0)
		currentDate := baseStart
		count := 0
		for count < 100 {
			if recModeRadio.Selected == "Interval" {
				n, _ := strconv.Atoi(recNumEntry.Text)
				if n < 1 {
					n = 1
				}
				switch recUnitSelect.Selected {
				case "Day(s)":
					currentDate = currentDate.AddDate(0, 0, n)
				case "Week(s)":
					currentDate = currentDate.AddDate(0, 0, n*7)
				case "Month(s)":
					currentDate = currentDate.AddDate(0, n, 0)
				case "Year(s)":
					currentDate = currentDate.AddDate(n, 0, 0)
				}
			} else {
				targetDayStr := recDaySelect.Selected
				targetWeekday := time.Monday
				switch targetDayStr {
				case "Sunday":
					targetWeekday = time.Sunday
				case "Monday":
					targetWeekday = time.Monday
				case "Tuesday":
					targetWeekday = time.Tuesday
				case "Wednesday":
					targetWeekday = time.Wednesday
				case "Thursday":
					targetWeekday = time.Thursday
				case "Friday":
					targetWeekday = time.Friday
				case "Saturday":
					targetWeekday = time.Saturday
				}
				daysToAdd := 0
				for {
					daysToAdd++
					d := currentDate.AddDate(0, 0, daysToAdd)
					if d.Weekday() == targetWeekday {
						currentDate = d
						break
					}
				}
				if recOrdinalSelect.Selected == "Every Other" {
					currentDate = currentDate.AddDate(0, 0, 7)
				}
			}
			if currentDate.After(limitDate) {
				break
			}
			newItem := baseItem
			newItem.ID = fmt.Sprintf("%d-%d", time.Now().UnixNano(), count)
			newItem.Start = currentDate.Format("2006-01-02 15:04")
			newItem.End = currentDate.Add(duration).Format("2006-01-02 15:04")
			itemsToCreate = append(itemsToCreate, newItem)
			count++
		}
	}

	items = append(items, itemsToCreate...)
	saveData()
	refreshCalendar()
	refreshKanban()
	sbTitleEntry.SetText("")
}

// --- STATE MANAGEMENT ---

func updateSidebarHeader() {
	mode := "Add New"
	if currentEditItemID != "" {
		mode = "Edit"
	}
	itemType := sbTypeSelect.Selected
	if itemType == "" {
		itemType = "Item"
	}
	sbHeaderLabel.SetText(fmt.Sprintf("%s %s", mode, itemType))
	if sbActionBtn != nil {
		if currentEditItemID == "" {
			sbActionBtn.Show()
		} else {
			sbActionBtn.Hide()
		}
	}
}

func startEditing(item *TodoItem) {
	currentEditItemID = item.ID
	sbCancelBtn.Show()
	sbDeleteBtn.Show()
	sbActionBtn.Hide()
	sbTitleEntry.SetText(item.Title)
	for _, g := range groups {
		if g.ID == item.GroupID {
			sbGroupSelect.SetSelected(g.Name)
			break
		}
	}
	sbTypeSelect.SetSelected(string(item.Type))
	s, _ := time.ParseInLocation("2006-01-02 15:04", item.Start, time.Local)
	e, _ := time.ParseInLocation("2006-01-02 15:04", item.End, time.Local)
	getTimeParts := func(t time.Time) (string, string, string) {
		h := t.Hour()
		ap := "AM"
		if h >= 12 {
			ap = "PM"
			if h > 12 {
				h -= 12
			}
		}
		if h == 0 {
			h = 12
		}
		return fmt.Sprintf("%02d", h), fmt.Sprintf("%02d", t.Minute()), ap
	}
	h, m, ap := getTimeParts(s)
	if item.Type == TypeTask {
		setTaskDate(s.Format("2006-01-02"))
		setTaskTime(h, m, ap)
	} else {
		setStartDate(s.Format("2006-01-02"))
		setStartTime(h, m, ap)
		eh, em, eap := getTimeParts(e)
		setEndDate(e.Format("2006-01-02"))
		setEndTime(eh, em, eap)
	}
	recCheck.SetChecked(false)
	recContainer.Hide()
	updateSidebarHeader()
}

func resetSidebar() {
	currentEditItemID = ""
	sbCancelBtn.Hide()
	sbDeleteBtn.Hide()
	sbActionBtn.Show()
	sbTitleEntry.SetText("")
	recCheck.SetChecked(false)
	recContainer.Hide()
	updateSidebarHeader()
}

// --- CALENDAR VIEW ---

func createCalendarArea() fyne.CanvasObject {
	btnPrev := widget.NewButton("<", func() { currentViewDate = currentViewDate.AddDate(0, -1, 0); refreshCalendar() })
	btnNext := widget.NewButton(">", func() { currentViewDate = currentViewDate.AddDate(0, 1, 0); refreshCalendar() })
	monthLabel = widget.NewLabel("")
	monthLabel.TextStyle = fyne.TextStyle{Bold: true}
	monthLabel.Alignment = fyne.TextAlignCenter
	nav := container.NewBorder(nil, nil, btnPrev, btnNext, monthLabel)
	headerGrid := container.NewGridWithColumns(7)
	for _, d := range []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"} {
		headerGrid.Add(widget.NewLabelWithStyle(d, fyne.TextAlignCenter, fyne.TextStyle{Bold: true}))
	}
	calendarGrid = container.NewGridWithColumns(7)
	refreshCalendar()
	return container.NewBorder(container.NewVBox(nav, headerGrid), nil, nil, nil, calendarGrid)
}

func refreshCalendar() {
	monthLabel.SetText(currentViewDate.Format("January 2006"))
	calendarGrid.Objects = nil
	groupColorMap := make(map[string]color.Color)
	for _, g := range groups {
		groupColorMap[g.ID] = parseHexColor(g.ColorHex)
	}
	year, month, _ := currentViewDate.Date()
	first := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	startOffset := int(first.Weekday())
	if startOffset == 0 {
		startOffset = 7
	}
	startOffset--
	daysInMonth := first.AddDate(0, 1, -1).Day()
	for i := 0; i < startOffset; i++ {
		calendarGrid.Add(layout.NewSpacer())
	}
	for d := 1; d <= daysInMonth; d++ {
		dayStart := time.Date(year, month, d, 0, 0, 0, 0, time.Local)
		dayEnd := time.Date(year, month, d, 23, 59, 59, 0, time.Local)
		bgCell := canvas.NewRectangle(color.Transparent)
		if dayStart.Year() == selectedCalendarDate.Year() && dayStart.Month() == selectedCalendarDate.Month() && dayStart.Day() == selectedCalendarDate.Day() {
			bgCell.FillColor = color.RGBA{80, 80, 80, 80}
			bgCell.StrokeColor = theme.PrimaryColor()
			bgCell.StrokeWidth = 2
		}
		cellContent := container.NewVBox(widget.NewLabelWithStyle(strconv.Itoa(d), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		for i := range items {
			item := &items[i]
			s, _ := time.ParseInLocation("2006-01-02 15:04", item.Start, time.Local)
			e, _ := time.ParseInLocation("2006-01-02 15:04", item.End, time.Local)
			if s.Before(dayEnd) && (e.After(dayStart) || e.Equal(dayStart)) {
				c, exists := groupColorMap[item.GroupID]
				if !exists {
					c = color.Gray{Y: 100}
				}
				if item.Completed {
					c = color.RGBA{200, 200, 200, 255}
				}
				var displayBlock *fyne.Container
				timeStr := s.Format("15:04")
				if item.Type == TypeEvent {
					timeStr = fmt.Sprintf("%s - %s", s.Format("15:04"), e.Format("15:04"))
				}
				if item.Type == TypeTask {
					displayText := fmt.Sprintf("• %s %s", timeStr, item.Title)
					if item.Completed {
						displayBlock = container.NewPadded(createStrikethroughText(displayText, c, 10))
					} else {
						displayBlock = container.NewPadded(canvas.NewText(displayText, c))
					}
				} else {
					bg := canvas.NewRectangle(c)
					bg.SetMinSize(fyne.NewSize(10, 16))
					eventText := fmt.Sprintf("%s (%s)", item.Title, timeStr)
					if item.Completed {
						displayBlock = container.NewStack(bg, container.NewPadded(createStrikethroughText(eventText, color.White, 10)))
					} else {
						displayBlock = container.NewStack(bg, container.NewPadded(canvas.NewText(eventText, color.White)))
					}
				}
				clickable := newClickableBox(displayBlock, func() { startEditing(item) })
				clickable.onRight = func(e *fyne.PointEvent) {
					statusLabel := "Mark Complete"
					if item.Completed {
						statusLabel = "Mark Incomplete"
					}
					widget.ShowPopUpMenuAtPosition(fyne.NewMenu("Actions", fyne.NewMenuItem(statusLabel, func() { item.Completed = !item.Completed; saveData(); refreshCalendar(); refreshKanban() }), fyne.NewMenuItemSeparator(), fyne.NewMenuItem("Move to...", func() { showMoveDialog(item) }), fyne.NewMenuItem("Delete", func() { performSmartDelete(item.ID) })), mainWindow.Canvas(), e.AbsolutePosition)
				}
				cellContent.Add(clickable)
			}
		}
		clickDateStr := dayStart.Format("2006-01-02")
		interactiveCell := newClickableBox(cellContent, func() {
			resetSidebar()
			selectedCalendarDate = dayStart
			if setTaskDate != nil {
				setTaskDate(clickDateStr)
			}
			if setStartDate != nil {
				setStartDate(clickDateStr)
			}
			if setEndDate != nil {
				setEndDate(clickDateStr)
			}
			refreshCalendar()
		})
		calendarGrid.Add(widget.NewCard("", "", container.NewStack(bgCell, interactiveCell)))
	}
}

// --- KANBAN VIEW ---

func createKanbanArea() fyne.CanvasObject {
	kanbanContainer = container.NewHBox()
	return container.NewHScroll(container.NewPadded(kanbanContainer))
}
func refreshKanban() {
	kanbanContainer.Objects = nil
	itemsByGroup := make(map[string][]*TodoItem)
	for i := range items {
		itemsByGroup[items[i].GroupID] = append(itemsByGroup[items[i].GroupID], &items[i])
	}
	for i := range groups {
		grp := &groups[i]
		grpColor := parseHexColor(grp.ColorHex)
		headerLabel := canvas.NewText(grp.Name, color.White)
		headerLabel.TextStyle = fyne.TextStyle{Bold: true}
		sortBtn := widget.NewButtonWithIcon("", theme.MenuIcon(), func() {
			widget.ShowPopUpMenuAtPosition(fyne.NewMenu("Sort", fyne.NewMenuItem("Sort by Date", func() { grp.SortMode = "date"; saveGroups(); refreshKanban() }), fyne.NewMenuItem("Sort A-Z", func() { grp.SortMode = "alpha"; saveGroups(); refreshKanban() })), mainWindow.Canvas(), fyne.CurrentApp().Driver().AbsolutePositionForObject(headerLabel))
		})
		headerBg := canvas.NewRectangle(grpColor)
		headerBg.SetMinSize(fyne.NewSize(250, 40))
		headerContent := container.NewBorder(nil, nil, nil, sortBtn, container.NewCenter(headerLabel))
		itemsBox := container.NewVBox()
		grpItems := itemsByGroup[grp.ID]
		sort.Slice(grpItems, func(a, b int) bool {
			if grpItems[a].Completed != grpItems[b].Completed {
				return !grpItems[a].Completed
			}
			if grp.SortMode == "alpha" {
				return strings.ToLower(grpItems[a].Title) < strings.ToLower(grpItems[b].Title)
			}
			return grpItems[a].Start < grpItems[b].Start
		})
		for _, item := range grpItems {
			cardBgColor := color.Color(color.RGBA{240, 240, 240, 255})
			textColor := color.Color(color.Black)
			if item.Completed {
				cardBgColor = color.RGBA{220, 220, 220, 255}
				textColor = color.RGBA{150, 150, 150, 255}
			}
			cardBg := canvas.NewRectangle(cardBgColor)
			cardBg.StrokeColor = color.RGBA{200, 200, 200, 255}
			cardBg.StrokeWidth = 1
			cardBg.CornerRadius = 5
			var titleObj fyne.CanvasObject
			if item.Completed {
				titleObj = createStrikethroughText(item.Title, textColor, 12)
			} else {
				t := canvas.NewText(item.Title, textColor)
				t.TextSize = 12
				titleObj = t
			}
			s, _ := time.ParseInLocation("2006-01-02 15:04", item.Start, time.Local)
			e, _ := time.ParseInLocation("2006-01-02 15:04", item.End, time.Local)
			dateStr := s.Format("Mon, Jan 02")
			timeInfo := s.Format("15:04")
			if item.Type == TypeEvent {
				timeInfo = fmt.Sprintf("%s - %s", s.Format("15:04"), e.Format("15:04"))
			}
			dateLabel := canvas.NewText(fmt.Sprintf("%s | %s", dateStr, timeInfo), color.RGBA{100, 100, 100, 255})
			dateLabel.TextSize = 10
			check := widget.NewCheck("", func(b bool) { item.Completed = b; saveData(); refreshCalendar(); refreshKanban() })
			check.Checked = item.Completed
			content := container.NewBorder(nil, nil, check, nil, container.NewVBox(titleObj, dateLabel))
			clickCard := newClickableBox(container.NewStack(cardBg, container.NewPadded(content)), func() { startEditing(item) })
			clickCard.onRight = func(e *fyne.PointEvent) {
				sl := "Mark Complete"
				if item.Completed {
					sl = "Mark Incomplete"
				}
				widget.ShowPopUpMenuAtPosition(fyne.NewMenu("Actions", fyne.NewMenuItem(sl, func() { item.Completed = !item.Completed; saveData(); refreshCalendar(); refreshKanban() }), fyne.NewMenuItemSeparator(), fyne.NewMenuItem("Move to...", func() { showMoveDialog(item) }), fyne.NewMenuItem("Delete", func() { performSmartDelete(item.ID) })), mainWindow.Canvas(), e.AbsolutePosition)
			}
			itemsBox.Add(clickCard)
		}
		kanbanContainer.Add(container.NewBorder(container.NewStack(headerBg, headerContent), nil, nil, nil, container.NewVScroll(container.NewPadded(itemsBox))))
		kanbanContainer.Add(layout.NewSpacer())
	}
	kanbanContainer.Refresh()
}
func showMoveDialog(item *TodoItem) {
	var d dialog.Dialog
	opts := []string{}
	for _, g := range groups {
		if g.ID != item.GroupID {
			opts = append(opts, g.Name)
		}
	}
	sel := widget.NewSelect(opts, nil)
	sel.PlaceHolder = "Select Group"
	btnConfirm := widget.NewButton("Move", func() {
		if sel.Selected == "" {
			return
		}
		for _, g := range groups {
			if g.Name == sel.Selected {
				item.GroupID = g.ID
				break
			}
		}
		saveData()
		refreshCalendar()
		refreshKanban()
		d.Hide()
	})
	d = dialog.NewCustom("Move Item", "Cancel", container.NewPadded(container.NewVBox(widget.NewLabel(fmt.Sprintf("Move '%s' to:", item.Title)), sel, btnConfirm)), mainWindow)
	d.Show()
}

// --- IMPORT / EXPORT / HELPERS ---

func showSettingsDialog() {
	var d dialog.Dialog

	themeSelect := widget.NewSelect([]string{"Light", "Dark"}, func(s string) {
		currentTheme = s
		if s == "Light" {
			myApp.Settings().SetTheme(theme.LightTheme())
		} else {
			myApp.Settings().SetTheme(theme.DarkTheme())
		}
	})
	themeSelect.SetSelected(currentTheme)

	calSelect := widget.NewSelect(availableCalendars, func(s string) {
		if s != activeCalendarName && s != "" {
			switchCalendar(s)
			d.Hide()
		}
	})
	calSelect.SetSelected(activeCalendarName)

	manageCalBtn := widget.NewButton("Create / Delete Calendars", func() { d.Hide(); showCalendarManager() })
	btnImport := widget.NewButtonWithIcon("Import .ICS", theme.FolderOpenIcon(), func() { importICS(); d.Hide() })
	btnExport := widget.NewButtonWithIcon("Export .ICS", theme.DocumentSaveIcon(), func() { exportICS() })

	content := container.NewVBox(
		widget.NewLabelWithStyle("App Settings", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		widget.NewLabel("Theme"), themeSelect,
		widget.NewSeparator(),
		widget.NewLabel("Active Calendar"), calSelect, manageCalBtn,
		widget.NewSeparator(),
		widget.NewLabel("Data Transfer"), container.NewGridWithColumns(2, btnImport, btnExport),
	)
	d = dialog.NewCustom("Settings", "Close", container.NewPadded(content), mainWindow)
	d.Resize(fyne.NewSize(400, 500))
	d.Show()
}

func getFilenames() (string, string) {
	prefix := strings.ReplaceAll(activeCalendarName, " ", "_")
	return prefix + "_data.json", prefix + "_groups.json"
}
func loadCalendarList() {
	file, err := os.ReadFile("calendars_meta.json")
	if err == nil {
		json.Unmarshal(file, &availableCalendars)
	}
	if len(availableCalendars) == 0 {
		availableCalendars = []string{"Default"}
		saveCalendarList()
	}
}
func saveCalendarList() {
	file, _ := json.MarshalIndent(availableCalendars, "", " ")
	_ = os.WriteFile("calendars_meta.json", file, 0644)
}
func switchCalendar(name string) {
	activeCalendarName = name
	items = []TodoItem{}
	groups = []Group{}
	loadGroups()
	loadData()
	refreshCalendar()
	refreshKanban()
	updateGroupDropdown()
	if len(groups) > 0 {
		sbGroupSelect.SetSelected(groups[0].Name)
	}
	resetSidebar()
}
func showCalendarManager() {
	var d dialog.Dialog
	input := widget.NewEntry()
	input.PlaceHolder = "New Calendar Name"
	list := widget.NewList(
		func() int { return len(availableCalendars) },
		func() fyne.CanvasObject {
			return container.NewHBox(widget.NewLabel("Name"), layout.NewSpacer(), widget.NewButtonWithIcon("", theme.DeleteIcon(), nil))
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			box := o.(*fyne.Container)
			lbl := box.Objects[0].(*widget.Label)
			btn := box.Objects[2].(*widget.Button)
			name := availableCalendars[i]
			lbl.SetText(name)
			btn.OnTapped = func() {
				if len(availableCalendars) <= 1 {
					dialog.ShowError(fmt.Errorf("cannot delete last"), mainWindow)
					return
				}
				dialog.ShowConfirm("Delete", "Delete '"+name+"'?", func(ok bool) {
					if ok {
						newList := []string{}
						for _, c := range availableCalendars {
							if c != name {
								newList = append(newList, c)
							}
						}
						availableCalendars = newList
						saveCalendarList()
						if activeCalendarName == name {
							switchCalendar(availableCalendars[0])
						}
						d.Hide()
						showCalendarManager()
					}
				}, mainWindow)
			}
		},
	)
	createBtn := widget.NewButton("Create", func() {
		if input.Text == "" {
			return
		}
		for _, c := range availableCalendars {
			if c == input.Text {
				return
			}
		}
		availableCalendars = append(availableCalendars, input.Text)
		saveCalendarList()
		switchCalendar(input.Text)
		d.Hide()
	})
	d = dialog.NewCustom("Manage Calendars", "Close", container.NewPadded(container.NewBorder(container.NewVBox(widget.NewLabel("Create New:"), container.NewBorder(nil, nil, nil, createBtn, input), widget.NewSeparator()), nil, nil, nil, list)), mainWindow)
	d.Resize(fyne.NewSize(400, 500))
	d.Show()
}
func showGroupManager() {
	var d dialog.Dialog
	listContainer := container.NewVBox()
	for i := range groups {
		grp := groups[i]
		colorRect := canvas.NewRectangle(parseHexColor(grp.ColorHex))
		colorRect.SetMinSize(fyne.NewSize(20, 20))
		lbl := widget.NewLabel(grp.Name)
		btnEdit := widget.NewButtonWithIcon("", theme.SettingsIcon(), func() { d.Hide(); showGroupForm(&grp) })
		btnDel := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
			dialog.ShowConfirm("Delete Group", "Delete '"+grp.Name+"'?", func(ok bool) {
				if ok {
					newGroups := []Group{}
					for _, g := range groups {
						if g.ID != grp.ID {
							newGroups = append(newGroups, g)
						}
					}
					groups = newGroups
					saveGroups()
					updateGroupDropdown()
					if sbGroupSelect.Selected == grp.Name {
						if len(groups) > 0 {
							sbGroupSelect.SetSelected(groups[0].Name)
						} else {
							sbGroupSelect.SetSelected("")
						}
					}
					refreshCalendar()
					refreshKanban()
					d.Hide()
					showGroupManager()
				}
			}, mainWindow)
		})
		listContainer.Add(container.NewBorder(nil, nil, colorRect, container.NewHBox(btnEdit, btnDel), lbl))
	}
	scroll := container.NewVScroll(listContainer)
	scroll.SetMinSize(fyne.NewSize(300, 400))
	d = dialog.NewCustom("Groups", "Close", scroll, mainWindow)
	d.Resize(fyne.NewSize(350, 500))
	d.Show()
}
func showGroupForm(existingGroup *Group) {
	var d dialog.Dialog
	isEdit := existingGroup != nil
	nameEntry := widget.NewEntry()
	nameEntry.PlaceHolder = "e.g. Work"
	defaultColor := PresetColors[0]
	if isEdit {
		nameEntry.SetText(existingGroup.Name)
		defaultColor = existingGroup.ColorHex
	}
	selectedColor := defaultColor
	previewRect := canvas.NewRectangle(parseHexColor(selectedColor))
	previewRect.SetMinSize(fyne.NewSize(50, 20))
	colorGrid := container.NewGridWithColumns(6)
	for _, c := range PresetColors {
		hex := c
		rect := canvas.NewRectangle(parseHexColor(hex))
		rect.SetMinSize(fyne.NewSize(30, 30))
		rect.StrokeColor = color.White
		rect.StrokeWidth = 1
		clickable := newClickableBox(container.NewStack(rect), func() { selectedColor = hex; previewRect.FillColor = parseHexColor(hex); previewRect.Refresh() })
		colorGrid.Add(clickable)
	}
	btnLabel := "Create Group"
	if isEdit {
		btnLabel = "Save Changes"
	}
	actionBtn := widget.NewButton(btnLabel, func() {
		if nameEntry.Text == "" {
			return
		}
		targetName := nameEntry.Text
		if isEdit {
			for i, g := range groups {
				if g.ID == existingGroup.ID {
					groups[i].Name = nameEntry.Text
					groups[i].ColorHex = selectedColor
					break
				}
			}
		} else {
			newGroup := Group{ID: fmt.Sprintf("g-%d", time.Now().UnixNano()), Name: nameEntry.Text, ColorHex: selectedColor}
			groups = append(groups, newGroup)
			targetName = newGroup.Name
		}
		saveGroups()
		updateGroupDropdown()
		refreshCalendar()
		refreshKanban()
		sbGroupSelect.SetSelected(targetName)
		if d != nil {
			d.Hide()
		}
	})
	content := container.NewVBox(widget.NewLabel("Group Name:"), nameEntry, widget.NewLabel("Group Color:"), previewRect, colorGrid, layout.NewSpacer(), actionBtn)
	d = dialog.NewCustom("Group", "Cancel", container.NewPadded(content), mainWindow)
	d.Resize(fyne.NewSize(300, 400))
	d.Show()
}
func performSmartDelete(targetID string) {
	var targetItem *TodoItem
	for i := range items {
		if items[i].ID == targetID {
			targetItem = &items[i]
			break
		}
	}
	if targetItem == nil {
		return
	}
	var d dialog.Dialog
	finish := func() {
		saveData()
		refreshCalendar()
		refreshKanban()
		resetSidebar()
		if d != nil {
			d.Hide()
		}
	}
	if targetItem.SeriesID == "" {
		dialog.ShowConfirm("Delete", "Delete?", func(ok bool) {
			if ok {
				newItems := []TodoItem{}
				for _, i := range items {
					if i.ID != targetID {
						newItems = append(newItems, i)
					}
				}
				items = newItems
				finish()
			}
		}, mainWindow)
		return
	}
	d = dialog.NewCustom("Delete Recurring", "Cancel", container.NewVBox(widget.NewLabel("Repeating item. Delete?"), widget.NewButton("This Only", func() {
		newItems := []TodoItem{}
		for _, i := range items {
			if i.ID != targetID {
				newItems = append(newItems, i)
			}
		}
		items = newItems
		finish()
	}), widget.NewButton("This + Future", func() {
		targetTime, _ := time.ParseInLocation("2006-01-02 15:04", targetItem.Start, time.Local)
		newItems := []TodoItem{}
		for _, i := range items {
			iTime, _ := time.ParseInLocation("2006-01-02 15:04", i.Start, time.Local)
			if i.SeriesID != targetItem.SeriesID || iTime.Before(targetTime) {
				newItems = append(newItems, i)
			}
		}
		items = newItems
		finish()
	}), widget.NewButton("All", func() {
		newItems := []TodoItem{}
		for _, i := range items {
			if i.SeriesID != targetItem.SeriesID {
				newItems = append(newItems, i)
			}
		}
		items = newItems
		finish()
	})), mainWindow)
	d.Show()
}
func importICS() {
	fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()
		data, _ := io.ReadAll(reader)
		parsedCal, err := ical.ParseCalendar(strings.NewReader(string(data)))
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to parse"), mainWindow)
			return
		}
		count := 0
		targetGroupID := ""
		if len(groups) > 0 {
			targetGroupID = groups[0].ID
		}
		for _, event := range parsedCal.Events() {
			sum := event.GetProperty(ical.ComponentPropertySummary)
			start := event.GetProperty(ical.ComponentPropertyDtStart)
			end := event.GetProperty(ical.ComponentPropertyDtEnd)
			if sum == nil || start == nil {
				continue
			}
			title := sum.Value
			sTime, err := time.Parse("20060102T150405", start.Value)
			if err != nil {
				sTime, _ = time.Parse("20060102", start.Value)
			}
			eTime := sTime
			if end != nil {
				eTime, _ = time.Parse("20060102T150405", end.Value)
				if eTime.IsZero() {
					eTime, _ = time.Parse("20060102", end.Value)
				}
			}
			iType := TypeTask
			if !eTime.Equal(sTime) && !eTime.IsZero() {
				iType = TypeEvent
			}
			items = append(items, TodoItem{ID: fmt.Sprintf("imp-%d-%d", time.Now().UnixNano(), count), Title: title, Start: sTime.Format("2006-01-02 15:04"), End: eTime.Format("2006-01-02 15:04"), Type: iType, GroupID: targetGroupID})
			count++
		}
		saveData()
		refreshCalendar()
		refreshKanban()
		dialog.ShowInformation("Imported", fmt.Sprintf("%d items", count), mainWindow)
	}, mainWindow)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".ics"}))
	fd.Show()
}
func exportICS() {
	cal := ical.NewCalendar()
	cal.SetMethod(ical.MethodPublish)
	gName := make(map[string]string)
	for _, g := range groups {
		gName[g.ID] = g.Name
	}
	for _, item := range items {
		evt := cal.AddEvent(item.ID)
		s, _ := time.ParseInLocation("2006-01-02 15:04", item.Start, time.Local)
		e, _ := time.ParseInLocation("2006-01-02 15:04", item.End, time.Local)
		evt.SetStartAt(s)
		evt.SetEndAt(e)
		evt.SetSummary(fmt.Sprintf("[%s] %s", gName[item.GroupID], item.Title))
	}
	saveDialog := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil || writer == nil {
			return
		}
		_, _ = writer.Write([]byte(cal.Serialize()))
		_ = writer.Close()
		dialog.ShowInformation("Success", "Exported", mainWindow)
	}, mainWindow)
	saveDialog.SetFileName("my_calendar.ics")
	saveDialog.Show()
}
func createDatePickerButton(parent fyne.Window, onChanged func(string)) (*widget.Button, func() string, func(string)) {
	selectedDate := time.Now()
	btn := widget.NewButton(selectedDate.Format("2006-01-02"), nil)
	setDate := func(dateStr string) {
		t, err := time.Parse("2006-01-02", dateStr)
		if err == nil {
			selectedDate = t
			btn.SetText(dateStr)
		}
	}
	btn.OnTapped = func() {
		navDate := selectedDate
		isUpdating := false
		startYear, endYear := 1900, 2100
		years := []string{}
		for i := startYear; i <= endYear; i++ {
			years = append(years, strconv.Itoa(i))
		}
		grid := container.NewGridWithColumns(7)
		yearList := widget.NewList(func() int { return len(years) }, func() fyne.CanvasObject {
			return widget.NewLabelWithStyle("2000", fyne.TextAlignCenter, fyne.TextStyle{})
		}, func(i widget.ListItemID, o fyne.CanvasObject) { o.(*widget.Label).SetText(years[i]) })
		monthSelect := widget.NewSelect([]string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}, nil)
		yearBtn := widget.NewButton("Year", nil)
		mainContent := container.NewStack(grid, container.NewScroll(yearList))
		var d dialog.Dialog
		var refreshPopup func()
		yearBtn.OnTapped = func() {
			if grid.Visible() {
				grid.Hide()
				yearList.Show()
				idx := navDate.Year() - startYear
				yearList.ScrollTo(idx)
				yearList.Select(idx)
			} else {
				yearList.Hide()
				grid.Show()
				refreshPopup()
			}
		}
		yearList.OnSelected = func(id widget.ListItemID) {
			navDate = time.Date(startYear+id, navDate.Month(), 1, 0, 0, 0, 0, time.Local)
			yearList.Hide()
			grid.Show()
			refreshPopup()
		}
		refreshPopup = func() {
			isUpdating = true
			monthSelect.SetSelectedIndex(int(navDate.Month()) - 1)
			yearBtn.SetText(strconv.Itoa(navDate.Year()))
			grid.Objects = nil
			y, m, _ := navDate.Date()
			first := time.Date(y, m, 1, 0, 0, 0, 0, time.Local)
			off := int(first.Weekday())
			if off == 0 {
				off = 7
			}
			off--
			dim := first.AddDate(0, 1, -1).Day()
			for _, day := range []string{"Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"} {
				grid.Add(widget.NewLabelWithStyle(day, fyne.TextAlignCenter, fyne.TextStyle{Bold: true}))
			}
			for i := 0; i < off; i++ {
				grid.Add(layout.NewSpacer())
			}
			for i := 1; i <= dim; i++ {
				dayNum := i
				dayBtn := widget.NewButton(strconv.Itoa(i), func() {
					selectedDate = time.Date(y, m, dayNum, 0, 0, 0, 0, time.Local)
					dateStr := selectedDate.Format("2006-01-02")
					btn.SetText(dateStr)
					if onChanged != nil {
						onChanged(dateStr)
					}
					if d != nil {
						d.Hide()
					}
				})
				if y == time.Now().Year() && m == time.Now().Month() && i == time.Now().Day() {
					dayBtn.Importance = widget.HighImportance
				}
				grid.Add(dayBtn)
			}
			isUpdating = false
			grid.Refresh()
		}
		btnPrev := widget.NewButton("<", func() { navDate = navDate.AddDate(0, -1, 0); refreshPopup() })
		btnNext := widget.NewButton(">", func() { navDate = navDate.AddDate(0, 1, 0); refreshPopup() })
		monthSelect.OnChanged = func(s string) {
			if isUpdating {
				return
			}
			for i, m := range monthSelect.Options {
				if m == s {
					navDate = navDate.AddDate(0, (i+1)-int(navDate.Month()), 0)
					refreshPopup()
					break
				}
			}
		}
		navBar := container.NewBorder(nil, nil, btnPrev, btnNext, container.NewGridWithColumns(2, monthSelect, yearBtn))
		yearList.Hide()
		refreshPopup()
		d = dialog.NewCustom("Select Date", "Cancel", container.NewPadded(container.NewBorder(navBar, nil, nil, nil, mainContent)), parent)
		d.Resize(fyne.NewSize(350, 400))
		d.Show()
	}
	return btn, func() string { return btn.Text }, setDate
}
func createTimePicker(onChange func()) (*widget.Select, *widget.Select, *widget.Select, *fyne.Container, func(string, string, string)) {
	hours := []string{}
	for i := 1; i <= 12; i++ {
		hours = append(hours, fmt.Sprintf("%02d", i))
	}
	mins := []string{}
	for i := 0; i < 60; i += 5 {
		mins = append(mins, fmt.Sprintf("%02d", i))
	}
	h := widget.NewSelect(hours, func(s string) {
		if onChange != nil {
			onChange()
		}
	})
	m := widget.NewSelect(mins, func(s string) {
		if onChange != nil {
			onChange()
		}
	})
	ap := widget.NewSelect([]string{"AM", "PM"}, func(s string) {
		if onChange != nil {
			onChange()
		}
	})
	h.SetSelected("09")
	m.SetSelected("00")
	ap.SetSelected("AM")
	setTime := func(hh, mm, ampm string) { h.SetSelected(hh); m.SetSelected(mm); ap.SetSelected(ampm) }
	return h, m, ap, container.NewGridWithColumns(3, h, m, ap), setTime
}
func updateGroupDropdown() {
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	options := []string{}
	for _, g := range groups {
		options = append(options, g.Name)
	}
	options = append(options, "+ Create New Group")
	sbGroupSelect.Options = options
	sbGroupSelect.Refresh()
}
func loadGroups() {
	_, groupFile := getFilenames()
	file, err := os.ReadFile(groupFile)
	if err == nil {
		_ = json.Unmarshal(file, &groups)
	}
	if len(groups) == 0 && os.IsNotExist(err) {
		groups = []Group{{"g-1", "Work", "#3498DB", ""}, {"g-2", "Personal", "#2ECC71", ""}}
		saveGroups()
	}
}
func saveGroups() {
	_, groupFile := getFilenames()
	file, _ := json.MarshalIndent(groups, "", " ")
	_ = os.WriteFile(groupFile, file, 0644)
}
func saveData() {
	dataFile, _ := getFilenames()
	file, _ := json.MarshalIndent(items, "", " ")
	_ = os.WriteFile(dataFile, file, 0644)
}
func loadData() {
	dataFile, _ := getFilenames()
	file, err := os.ReadFile(dataFile)
	if err == nil {
		_ = json.Unmarshal(file, &items)
		for i := range items {
			if items[i].GroupID == "" && items[i].GroupName != "" {
				for _, g := range groups {
					if g.Name == items[i].GroupName {
						items[i].GroupID = g.ID
						break
					}
				}
			}
		}
	}
}
func parseHexColor(s string) color.Color {
	if len(s) > 0 && s[0] == '#' {
		s = s[1:]
	}
	if len(s) != 6 {
		return color.RGBA{128, 128, 128, 255}
	}
	r, _ := strconv.ParseUint(s[0:2], 16, 8)
	g, _ := strconv.ParseUint(s[2:4], 16, 8)
	b, _ := strconv.ParseUint(s[4:6], 16, 8)
	return color.RGBA{uint8(r), uint8(g), uint8(b), 255}
}
