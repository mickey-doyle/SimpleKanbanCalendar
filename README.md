# Simple Kanban Calendar

Small offline desktop application for task and events management. Built with Go & Fyne, this application provides a calendar interface along with a Kanban board.

## Features

Dual Views: Change between a monthly calendar view, a Kanban board, or other views.
- Task & Event Management: To-do’s or events marked with time blocking.
- Recurring Items: Allow entries to repeat on a daily, weekly, monthly, or user-specified basis.
- Project Management: Use color-coded columns for task grouping.
- Smart Sorting: Sorting Kanban columns based on Date, Alphabetical, or Status.
- Data Privacy: Data is stored entirely on your computer in JSON files (todo_data.json and groups.json). Cloud is not required.
- Import/Export: Import/export files in the .ics (iCalendar) format to connect with Google or Apple Calendar.
- Themes: Implements Dark Mode Support, Light Mode Support.

## How to Install (Windows Only)

### Option 1: Installation Wizard
If you simply wish to use the application, download the installer from the Releases page.

1. Click on the Releases tab within this repository.

2. Download the newest SimpleKanban

3. Open the file that will run the Setup Wizard.

4. Click on the application from your desktop or start menu.

---

### Option 2: Build from Source

If you’re a developer or want to compile it yourself, follow these steps.

#### Prerequisites
- Go (Golang) 1.20 or higher
- GCC Compiler (Required for graphics rendering with the Fyne library) e.g. TDM-GCC or MinGW for Windows
- Fyne Toolkit: `go install fyne.io/fyne`

#### Build Steps
1. Clone the repo:
```
git clone https://github.com/mickey-doyle/SimpleKanbanCalendar.git
cd SimpleKanbanCalendar
```
2. Install dependencies:
```
go mod tidy
```
3. Running in Developer Mode:
```
go run main.go
```
4. Build for Windows (no console window):
```  
go build -ldflags -H=windowsgui -o SimpleKanbanCalendar.exe
```
5. Package with icon (if you have the Fyne CLI):
``` 
fyne package -os windows -icon Icon.png --name "Simple Kanban Calendar"
```

## Creating the Installer 

If you have modified the code and want to generate a new `SimpleKanbanCalendar.exe` installer:

1.  Ensure you have **Inno Setup Compiler** installed.
2.  Run the `fyne package` command (step 5 above) to generate the executable.
3.  Open `Installer Script.iss` in Inno Setup.
4.  Click **Build > Compile**.
5.  The new installer will be generated in the project folder.
