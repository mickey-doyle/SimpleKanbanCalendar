# Simple Kanban Calendar

A lightweight, offline, and private desktop application for managing tasks and events. Built with **Go** and **Fyne**, this application combines a traditional Calendar view with a Kanban board for efficient project management.

## 🚀 Features

* **Dual Views:** Seamlessly switch between a Monthly Calendar and a Kanban Board.
* **Task & Event Management:** Create simple to-do items or time-blocked events.
* **Recurring Items:** Set tasks to repeat daily, weekly, monthly, or on custom intervals.
* **Project Management:** Organize tasks into color-coded Groups/Columns.
* **Smart Sorting:** Sort Kanban columns by Date, Alphabetical order, or Status.
* **Data Privacy:** All data is stored locally in JSON files (`todo_data.json` and `groups.json`). No cloud accounts required.
* **Import/Export:** Full support for importing and exporting `.ics` (iCalendar) files to sync with Google/Apple Calendar.
* **Themes:** Native Dark Mode and Light Mode support.

---

## 📥 How to Install (Windows)

### Option 1: The Easy Way (Recommended)
If you just want to use the application, grab the installer from our Releases page.

1.  Go to the **[Releases](../../releases)** section of this repository.
2.  Download the latest **`MyCalendarSetup.exe`**.
3.  Double-click the file to launch the Setup Wizard.
4.  (Optional) Check the box to associate `.ics` files if you want this to be your default calendar app.
5.  Launch the app from your Desktop or Start Menu!

---

### Option 2: Build from Source
If you are a developer or want to compile the code yourself, follow these steps.

#### Prerequisites
* **Go (Golang):** Version 1.20 or higher.
* **GCC Compiler:** Required by Fyne for graphics rendering (e.g., TDM-GCC or MinGW on Windows).
* **Fyne Toolkit:** `go install fyne.io/fyne/v2/cmd/fyne@latest`

#### Build Steps
1.  **Clone the repository:**
    ```bash
    git clone [https://github.com/YOUR_USERNAME/gocalendar.git](https://github.com/YOUR_USERNAME/gocalendar.git)
    cd gocalendar
    ```

2.  **Install dependencies:**
    ```bash
    go mod tidy
    ```

3.  **Run in Developer Mode:**
    ```bash
    go run main.go
    ```

4.  **Build for Windows (No Console Window):**
    ```bash
    go build -ldflags -H=windowsgui -o GoCalendar.exe
    ```

5.  **Package with Icon:**
    If you have the Fyne command line tool installed:
    ```bash
    fyne package -os windows -icon Icon.png --name "My Calendar App"
    ```

---

## 🛠️ Creating the Installer
If you have modified the code and want to generate a new `MyCalendarSetup.exe` installer:

1.  Ensure you have **Inno Setup Compiler** installed.
2.  Run the `fyne package` command (step 5 above) to generate the executable.
3.  Open `setup_script.iss` in Inno Setup.
4.  Click **Build > Compile**.
5.  The new installer will be generated in the project folder.

---

## 🎮 Usage Tips

* **Right-Click** any task on the Calendar or Kanban board to access the context menu (Delete, Move to Group, Mark Complete).
* **Click** a date box on the Calendar to instantly start adding a task for that specific day.
* **Manage Groups** via the button in the sidebar to change column colors or rename workflow stages.
* **Settings** (Top Right Icon) allows you to toggle themes and manage multiple calendar profiles (e.g., separate Work vs. Personal databases).

## 📄 License
This project is open source.
