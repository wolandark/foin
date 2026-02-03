package main

/*
#cgo pkg-config: gtk+-3.0 vte-2.91
#include <gtk/gtk.h>
#include <vte/vte.h>
#include <stdlib.h>

extern void goCommitCallback(char *text, guint length);
extern gboolean goButtonPressCallback(GdkEventButton *event);
extern gboolean goKeyPressCallback(GdkEventKey *event);

static void on_commit(VteTerminal *terminal, gchar *text, guint size, gpointer user_data) {
    goCommitCallback((char*)text, size);
}

static gboolean on_button_press(GtkWidget *widget, GdkEventButton *event, gpointer user_data) {
    return goButtonPressCallback(event);
}

static gboolean on_key_press(GtkWidget *widget, GdkEventKey *event, gpointer user_data) {
    return goKeyPressCallback(event);
}

static GtkWidget* create_vte_terminal() {
    GtkWidget *term = vte_terminal_new();
    g_signal_connect(term, "commit", G_CALLBACK(on_commit), NULL);
    g_signal_connect(term, "button-press-event", G_CALLBACK(on_button_press), NULL);
    g_signal_connect(term, "key-press-event", G_CALLBACK(on_key_press), NULL);
    return term;
}

static void vte_spawn_shell(VteTerminal *terminal, const char *working_dir) {
    char *argv[] = {NULL, NULL};
    argv[0] = vte_get_user_shell();
    if (argv[0] == NULL) {
        argv[0] = "/bin/bash";
    }

    vte_terminal_spawn_async(
        terminal,
        VTE_PTY_DEFAULT,
        working_dir,
        argv,
        NULL,
        G_SPAWN_DEFAULT,
        NULL,
        NULL,
        NULL,
        -1,
        NULL,
        NULL,
        NULL
    );
}

static void vte_spawn_command(VteTerminal *terminal, const char *working_dir, char **argv) {
    vte_terminal_spawn_async(
        terminal,
        VTE_PTY_DEFAULT,
        working_dir,
        argv,
        NULL,
        G_SPAWN_DEFAULT,
        NULL,
        NULL,
        NULL,
        -1,
        NULL,
        NULL,
        NULL
    );
}

static void vte_set_font_from_string(VteTerminal *terminal, const char *font_desc) {
    PangoFontDescription *font = pango_font_description_from_string(font_desc);
    vte_terminal_set_font(terminal, font);
    pango_font_description_free(font);
}

static void vte_set_colors(VteTerminal *terminal, const char *fg, const char *bg) {
    GdkRGBA foreground, background;
    gdk_rgba_parse(&foreground, fg);
    gdk_rgba_parse(&background, bg);
    vte_terminal_set_colors(terminal, &foreground, &background, NULL, 0);
}

static void vte_set_scrollback(VteTerminal *terminal, glong lines) {
    vte_terminal_set_scrollback_lines(terminal, lines);
}

static void vte_copy_clipboard(VteTerminal *terminal) {
    vte_terminal_copy_clipboard_format(terminal, VTE_FORMAT_TEXT);
}

static void vte_paste_clipboard(VteTerminal *terminal) {
    vte_terminal_paste_clipboard(terminal);
}

static void vte_select_all(VteTerminal *terminal) {
    vte_terminal_select_all(terminal);
}

static void vte_feed_child_text(VteTerminal *terminal, const char *text) {
    vte_terminal_feed_child(terminal, text, -1);
}

static guint get_event_button(GdkEventButton *event) {
    return event->button;
}

static guint get_event_keyval(GdkEventKey *event) {
    return event->keyval;
}

static guint get_event_state(GdkEventKey *event) {
    return event->state;
}

static void popup_menu_at_pointer(GtkMenu *menu) {
    gtk_menu_popup(menu, NULL, NULL, NULL, NULL, 3, gtk_get_current_event_time());
}
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"unsafe"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

type SSHHost struct {
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Password string `json:"password,omitempty"`
	KeyFile  string `json:"keyfile,omitempty"`
	Auto     bool   `json:"auto,omitempty"`
}

type Config struct {
	Font       string `json:"font"`
	FontSize   int    `json:"font_size"`
	Foreground string `json:"foreground"`
	Background string `json:"background"`
	Scrollback int64  `json:"scrollback"`
}

type App struct {
	window       *gtk.Window
	terminal     *C.GtkWidget
	vte          *C.VteTerminal
	hostList     *gtk.ListBox
	hosts        []SSHHost
	config       Config
	configPath   string
	hostsPath    string
	sshRegex     *regexp.Regexp
	mu           sync.Mutex
	currentInput strings.Builder
}

var app *App

func main() {
	gtk.Init(nil)

	app = &App{
		sshRegex: regexp.MustCompile(`ssh\s+(?:(?:-[A-Za-z]+\s+\S+\s+)*)?(?:([^@\s]+)@)?([A-Za-z0-9][-A-Za-z0-9.]+)(?:\s+-p\s+(\d+))?`),
	}

	if err := app.loadPaths(); err != nil {
		fmt.Fprintf(os.Stderr, "Error setting up paths: %v\n", err)
		os.Exit(1)
	}

	app.loadConfig()
	app.loadHosts()

	if err := app.createUI(); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating UI: %v\n", err)
		os.Exit(1)
	}

	app.window.ShowAll()
	gtk.Main()
}

func (a *App) loadPaths() error {
	usr, err := user.Current()
	if err != nil {
		return err
	}

	configDir := filepath.Join(usr.HomeDir, ".config", "foin")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	a.configPath = filepath.Join(configDir, "config.json")
	a.hostsPath = filepath.Join(configDir, "hosts.json")
	return nil
}

func (a *App) loadConfig() {
	a.config = Config{
		Font:       "Monospace",
		FontSize:   12,
		Foreground: "#D4D4D4",
		Background: "#1E1E1E",
		Scrollback: 10000,
	}

	data, err := os.ReadFile(a.configPath)
	if err == nil {
		json.Unmarshal(data, &a.config)
	}
}

func (a *App) saveConfig() error {
	data, err := json.MarshalIndent(a.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.configPath, data, 0644)
}

func (a *App) loadHosts() {
	a.hosts = []SSHHost{}
	data, err := os.ReadFile(a.hostsPath)
	if err == nil {
		json.Unmarshal(data, &a.hosts)
	}
}

func (a *App) saveHosts() error {
	data, err := json.MarshalIndent(a.hosts, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.hostsPath, data, 0644)
}

func (a *App) createUI() error {
	var err error

	a.window, err = gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		return err
	}
	a.window.SetTitle("Foin Terminal")
	a.window.SetDefaultSize(1200, 700)
	a.window.Connect("destroy", func() {
		gtk.MainQuit()
	})

	mainBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)

	sidebar, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	sidebar.SetSizeRequest(220, -1)

	sidebarStyle, _ := sidebar.GetStyleContext()
	cssProvider, _ := gtk.CssProviderNew()
	cssProvider.LoadFromData(`
		.sidebar {
			background-color: #252526;
			border-right: 1px solid #3C3C3C;
		}
		.sidebar-header {
			background-color: #2D2D30;
			padding: 8px 12px;
			border-bottom: 1px solid #3C3C3C;
		}
		.sidebar-header label {
			color: #CCCCCC;
			font-weight: bold;
			font-size: 13px;
		}
		.add-button {
			background: transparent;
			border: none;
			color: #4EC9B0;
			min-width: 28px;
			min-height: 28px;
			padding: 0;
		}
		.add-button:hover {
			background-color: #3C3C3C;
		}
		.host-row {
			padding: 8px 12px;
			border-bottom: 1px solid #2D2D30;
		}
		.host-row:hover {
			background-color: #2A2D2E;
		}
		.host-row:selected {
			background-color: #094771;
		}
		.host-name {
			color: #4EC9B0;
			font-weight: 500;
		}
		.host-detail {
			color: #808080;
			font-size: 11px;
		}
		.delete-btn {
			background: transparent;
			border: none;
			color: #F14C4C;
			min-width: 20px;
			min-height: 20px;
			padding: 2px;
			opacity: 0.6;
		}
		.delete-btn:hover {
			opacity: 1;
			background-color: rgba(241, 76, 76, 0.2);
		}
	`)
	screen, _ := gdk.ScreenGetDefault()
	gtk.AddProviderForScreen(screen, cssProvider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
	sidebarStyle.AddClass("sidebar")

	headerBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	headerStyle, _ := headerBox.GetStyleContext()
	headerStyle.AddClass("sidebar-header")

	headerLabel, _ := gtk.LabelNew("SSH Sessions")
	headerBox.PackStart(headerLabel, true, true, 0)

	addButton, _ := gtk.ButtonNewFromIconName("list-add-symbolic", gtk.ICON_SIZE_BUTTON)
	addStyle, _ := addButton.GetStyleContext()
	addStyle.AddClass("add-button")
	addButton.Connect("clicked", a.showAddHostDialog)
	headerBox.PackEnd(addButton, false, false, 0)

	sidebar.PackStart(headerBox, false, false, 0)

	scrolled, _ := gtk.ScrolledWindowNew(nil, nil)
	scrolled.SetPolicy(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC)

	a.hostList, _ = gtk.ListBoxNew()
	a.hostList.SetSelectionMode(gtk.SELECTION_SINGLE)
	a.hostList.Connect("row-activated", a.onHostActivated)
	scrolled.Add(a.hostList)

	sidebar.PackStart(scrolled, true, true, 0)
	mainBox.PackStart(sidebar, false, false, 0)

	termBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)

	a.terminal = C.create_vte_terminal()
	a.vte = (*C.VteTerminal)(unsafe.Pointer(a.terminal))

	termWidget := glib.Take(unsafe.Pointer(a.terminal))
	termGtk := &gtk.Widget{glib.InitiallyUnowned{termWidget}}

	a.applyTerminalSettings()

	homeDir, _ := os.UserHomeDir()
	cHomeDir := C.CString(homeDir)
	defer C.free(unsafe.Pointer(cHomeDir))
	C.vte_spawn_shell(a.vte, cHomeDir)

	termGtk.SetHExpand(true)
	termGtk.SetVExpand(true)
	termBox.PackStart(termGtk, true, true, 0)

	mainBox.PackStart(termBox, true, true, 0)

	a.window.Add(mainBox)
	a.refreshHostList()

	return nil
}

func (a *App) applyTerminalSettings() {
	fontDesc := fmt.Sprintf("%s %d", a.config.Font, a.config.FontSize)
	cFont := C.CString(fontDesc)
	defer C.free(unsafe.Pointer(cFont))
	C.vte_set_font_from_string(a.vte, cFont)

	cFg := C.CString(a.config.Foreground)
	cBg := C.CString(a.config.Background)
	defer C.free(unsafe.Pointer(cFg))
	defer C.free(unsafe.Pointer(cBg))
	C.vte_set_colors(a.vte, cFg, cBg)

	C.vte_set_scrollback(a.vte, C.glong(a.config.Scrollback))
}

func (a *App) refreshHostList() {
	children := a.hostList.GetChildren()
	children.Foreach(func(item interface{}) {
		if widget, ok := item.(*gtk.Widget); ok {
			widget.Destroy()
		}
	})

	for i, host := range a.hosts {
		row := a.createHostRow(host, i)
		a.hostList.Add(row)
	}
	a.hostList.ShowAll()
}

func (a *App) createHostRow(host SSHHost, index int) *gtk.ListBoxRow {
	row, _ := gtk.ListBoxRowNew()
	rowStyle, _ := row.GetStyleContext()
	rowStyle.AddClass("host-row")

	hbox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 8)
	hbox.SetMarginStart(8)
	hbox.SetMarginEnd(8)
	hbox.SetMarginTop(6)
	hbox.SetMarginBottom(6)

	vbox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 2)

	displayName := host.Name
	if displayName == "" {
		displayName = host.Host
	}
	nameLabel, _ := gtk.LabelNew(displayName)
	nameLabel.SetHAlign(gtk.ALIGN_START)
	nameStyle, _ := nameLabel.GetStyleContext()
	nameStyle.AddClass("host-name")

	detail := fmt.Sprintf("%s@%s", host.User, host.Host)
	if host.Port != "" && host.Port != "22" {
		detail += ":" + host.Port
	}
	if host.Auto {
		detail += " (auto)"
	}
	detailLabel, _ := gtk.LabelNew(detail)
	detailLabel.SetHAlign(gtk.ALIGN_START)
	detailStyle, _ := detailLabel.GetStyleContext()
	detailStyle.AddClass("host-detail")

	vbox.PackStart(nameLabel, false, false, 0)
	vbox.PackStart(detailLabel, false, false, 0)

	hbox.PackStart(vbox, true, true, 0)

	deleteBtn, _ := gtk.ButtonNewFromIconName("edit-delete-symbolic", gtk.ICON_SIZE_BUTTON)
	deleteStyle, _ := deleteBtn.GetStyleContext()
	deleteStyle.AddClass("delete-btn")
	deleteBtn.Connect("clicked", func() {
		a.deleteHost(index)
	})
	hbox.PackEnd(deleteBtn, false, false, 0)

	row.Add(hbox)
	return row
}

func (a *App) deleteHost(index int) {
	dialog := gtk.MessageDialogNew(
		a.window,
		gtk.DIALOG_MODAL,
		gtk.MESSAGE_QUESTION,
		gtk.BUTTONS_YES_NO,
		"Delete this SSH host?",
	)
	defer dialog.Destroy()

	response := dialog.Run()
	if response == gtk.RESPONSE_YES {
		a.mu.Lock()
		a.hosts = append(a.hosts[:index], a.hosts[index+1:]...)
		a.mu.Unlock()
		a.saveHosts()
		a.refreshHostList()
	}
}

func (a *App) onHostActivated(listBox *gtk.ListBox, row *gtk.ListBoxRow) {
	index := row.GetIndex()
	if index < 0 || index >= len(a.hosts) {
		return
	}

	host := a.hosts[index]
	a.connectToHost(host)
}

func (a *App) connectToHost(host SSHHost) {
	var cmd strings.Builder

	if host.Password != "" {
		cmd.WriteString("sshpass -p '")
		cmd.WriteString(strings.ReplaceAll(host.Password, "'", "'\\''"))
		cmd.WriteString("' ")
	}

	cmd.WriteString("ssh ")

	if host.KeyFile != "" {
		cmd.WriteString("-i ")
		cmd.WriteString(host.KeyFile)
		cmd.WriteString(" ")
	}

	if host.Port != "" && host.Port != "22" {
		cmd.WriteString("-p ")
		cmd.WriteString(host.Port)
		cmd.WriteString(" ")
	}

	if host.User != "" {
		cmd.WriteString(host.User)
		cmd.WriteString("@")
	}
	cmd.WriteString(host.Host)
	cmd.WriteString("\n")

	cmdStr := cmd.String()
	cCmd := C.CString(cmdStr)
	defer C.free(unsafe.Pointer(cCmd))
	C.vte_feed_child_text(a.vte, cCmd)
}

func (a *App) showAddHostDialog() {
	dialog, _ := gtk.DialogNew()
	dialog.SetTransientFor(a.window)
	dialog.SetModal(true)
	dialog.SetTitle("Add SSH Host")
	dialog.AddButton("Cancel", gtk.RESPONSE_CANCEL)
	dialog.AddButton("Add", gtk.RESPONSE_OK)
	dialog.SetDefaultSize(400, 300)

	contentArea, _ := dialog.GetContentArea()
	contentArea.SetMarginStart(20)
	contentArea.SetMarginEnd(20)
	contentArea.SetMarginTop(20)
	contentArea.SetMarginBottom(10)
	contentArea.SetSpacing(12)

	grid, _ := gtk.GridNew()
	grid.SetColumnSpacing(12)
	grid.SetRowSpacing(8)

	nameLabel, _ := gtk.LabelNew("Name:")
	nameLabel.SetHAlign(gtk.ALIGN_END)
	nameEntry, _ := gtk.EntryNew()
	nameEntry.SetPlaceholderText("Optional display name")
	grid.Attach(nameLabel, 0, 0, 1, 1)
	grid.Attach(nameEntry, 1, 0, 1, 1)

	hostLabel, _ := gtk.LabelNew("Host:")
	hostLabel.SetHAlign(gtk.ALIGN_END)
	hostEntry, _ := gtk.EntryNew()
	hostEntry.SetPlaceholderText("IP or hostname")
	grid.Attach(hostLabel, 0, 1, 1, 1)
	grid.Attach(hostEntry, 1, 1, 1, 1)

	portLabel, _ := gtk.LabelNew("Port:")
	portLabel.SetHAlign(gtk.ALIGN_END)
	portEntry, _ := gtk.EntryNew()
	portEntry.SetText("22")
	grid.Attach(portLabel, 0, 2, 1, 1)
	grid.Attach(portEntry, 1, 2, 1, 1)

	userLabel, _ := gtk.LabelNew("User:")
	userLabel.SetHAlign(gtk.ALIGN_END)
	userEntry, _ := gtk.EntryNew()
	currentUser, _ := user.Current()
	userEntry.SetText(currentUser.Username)
	grid.Attach(userLabel, 0, 3, 1, 1)
	grid.Attach(userEntry, 1, 3, 1, 1)

	passLabel, _ := gtk.LabelNew("Password:")
	passLabel.SetHAlign(gtk.ALIGN_END)
	passEntry, _ := gtk.EntryNew()
	passEntry.SetVisibility(false)
	passEntry.SetPlaceholderText("Leave empty for key-based auth")
	grid.Attach(passLabel, 0, 4, 1, 1)
	grid.Attach(passEntry, 1, 4, 1, 1)

	keyLabel, _ := gtk.LabelNew("Key File:")
	keyLabel.SetHAlign(gtk.ALIGN_END)
	keyBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 4)
	keyEntry, _ := gtk.EntryNew()
	keyEntry.SetPlaceholderText("Path to private key")
	keyEntry.SetHExpand(true)
	browseBtn, _ := gtk.ButtonNewWithLabel("Browse")
	browseBtn.Connect("clicked", func() {
		fc, _ := gtk.FileChooserDialogNewWith2Buttons(
			"Select Key File",
			a.window,
			gtk.FILE_CHOOSER_ACTION_OPEN,
			"Cancel", gtk.RESPONSE_CANCEL,
			"Open", gtk.RESPONSE_ACCEPT,
		)
		sshDir := filepath.Join(currentUser.HomeDir, ".ssh")
		fc.SetCurrentFolder(sshDir)
		if fc.Run() == gtk.RESPONSE_ACCEPT {
			keyEntry.SetText(fc.GetFilename())
		}
		fc.Destroy()
	})
	keyBox.PackStart(keyEntry, true, true, 0)
	keyBox.PackStart(browseBtn, false, false, 0)
	grid.Attach(keyLabel, 0, 5, 1, 1)
	grid.Attach(keyBox, 1, 5, 1, 1)

	contentArea.Add(grid)
	dialog.ShowAll()

	response := dialog.Run()
	if response == gtk.RESPONSE_OK {
		hostText, _ := hostEntry.GetText()
		if hostText != "" {
			nameText, _ := nameEntry.GetText()
			portText, _ := portEntry.GetText()
			userText, _ := userEntry.GetText()
			passText, _ := passEntry.GetText()
			keyText, _ := keyEntry.GetText()

			newHost := SSHHost{
				Name:     nameText,
				Host:     hostText,
				Port:     portText,
				User:     userText,
				Password: passText,
				KeyFile:  keyText,
				Auto:     false,
			}

			a.mu.Lock()
			a.hosts = append(a.hosts, newHost)
			a.mu.Unlock()
			a.saveHosts()
			a.refreshHostList()
		}
	}
	dialog.Destroy()
}

//export goButtonPressCallback
func goButtonPressCallback(event *C.GdkEventButton) C.gboolean {
	button := C.get_event_button(event)
	if button == 3 {
		glib.IdleAdd(func() {
			app.showContextMenuAt()
		})
		return C.TRUE
	}
	return C.FALSE
}

//export goKeyPressCallback
func goKeyPressCallback(event *C.GdkEventKey) C.gboolean {
	keyval := C.get_event_keyval(event)
	state := C.get_event_state(event)

	ctrlShift := C.guint(gdk.CONTROL_MASK | gdk.SHIFT_MASK)

	if state&ctrlShift == ctrlShift {
		switch keyval {
		case C.guint(gdk.KEY_C):
			C.vte_copy_clipboard(app.vte)
			return C.TRUE
		case C.guint(gdk.KEY_V):
			C.vte_paste_clipboard(app.vte)
			return C.TRUE
		}
	}

	return C.FALSE
}

//export goCommitCallback
func goCommitCallback(text *C.char, length C.guint) {
	goText := C.GoStringN(text, C.int(length))

	var cmdToCheck string

	app.mu.Lock()
	for _, ch := range goText {
		if ch == '\r' || ch == '\n' {
			cmdToCheck = strings.TrimSpace(app.currentInput.String())
			app.currentInput.Reset()
		} else if ch == '\x7f' || ch == '\b' {
			s := app.currentInput.String()
			if len(s) > 0 {
				app.currentInput.Reset()
				app.currentInput.WriteString(s[:len(s)-1])
			}
		} else if ch >= 32 && ch < 127 {
			app.currentInput.WriteRune(ch)
		}
	}
	app.mu.Unlock()

	if cmdToCheck != "" {
		go app.checkForSSHCommand(cmdToCheck)
	}
}

func (a *App) checkForSSHCommand(cmd string) {
	matches := a.sshRegex.FindStringSubmatch(cmd)
	if matches == nil {
		return
	}

	var sshUser, host, port string

	if matches[1] != "" {
		sshUser = matches[1]
	} else {
		currentUser, _ := user.Current()
		sshUser = currentUser.Username
	}

	host = matches[2]
	if host == "" {
		return
	}

	if len(matches) > 3 && matches[3] != "" {
		port = matches[3]
	} else {
		port = "22"
	}

	a.mu.Lock()
	for _, h := range a.hosts {
		if h.Host == host && h.User == sshUser {
			a.mu.Unlock()
			return
		}
	}

	newHost := SSHHost{
		Name: "",
		Host: host,
		Port: port,
		User: sshUser,
		Auto: true,
	}
	a.hosts = append(a.hosts, newHost)
	a.mu.Unlock()

	a.saveHosts()
	glib.IdleAdd(a.refreshHostList)
}

func (a *App) showContextMenuAt() {
	menu, _ := gtk.MenuNew()

	copyItem, _ := gtk.MenuItemNewWithLabel("Copy")
	copyItem.Connect("activate", func() {
		C.vte_copy_clipboard(a.vte)
	})
	menu.Append(copyItem)

	pasteItem, _ := gtk.MenuItemNewWithLabel("Paste")
	pasteItem.Connect("activate", func() {
		C.vte_paste_clipboard(a.vte)
	})
	menu.Append(pasteItem)

	selectAllItem, _ := gtk.MenuItemNewWithLabel("Select All")
	selectAllItem.Connect("activate", func() {
		C.vte_select_all(a.vte)
	})
	menu.Append(selectAllItem)

	sep1, _ := gtk.SeparatorMenuItemNew()
	menu.Append(sep1)

	settingsItem, _ := gtk.MenuItemNewWithLabel("Settings")
	settingsItem.Connect("activate", a.showSettingsDialog)
	menu.Append(settingsItem)

	menu.ShowAll()
	C.popup_menu_at_pointer((*C.GtkMenu)(unsafe.Pointer(menu.Native())))
}

func (a *App) showSettingsDialog() {
	dialog, _ := gtk.DialogNew()
	dialog.SetTransientFor(a.window)
	dialog.SetModal(true)
	dialog.SetTitle("Settings")
	dialog.AddButton("Cancel", gtk.RESPONSE_CANCEL)
	dialog.AddButton("Apply", gtk.RESPONSE_OK)
	dialog.SetDefaultSize(450, 350)

	contentArea, _ := dialog.GetContentArea()
	contentArea.SetMarginStart(20)
	contentArea.SetMarginEnd(20)
	contentArea.SetMarginTop(20)
	contentArea.SetMarginBottom(10)
	contentArea.SetSpacing(16)

	notebook, _ := gtk.NotebookNew()

	appearanceGrid, _ := gtk.GridNew()
	appearanceGrid.SetColumnSpacing(12)
	appearanceGrid.SetRowSpacing(12)
	appearanceGrid.SetMarginStart(12)
	appearanceGrid.SetMarginEnd(12)
	appearanceGrid.SetMarginTop(12)

	fontLabel, _ := gtk.LabelNew("Font Family:")
	fontLabel.SetHAlign(gtk.ALIGN_END)
	fontEntry, _ := gtk.EntryNew()
	fontEntry.SetText(a.config.Font)
	appearanceGrid.Attach(fontLabel, 0, 0, 1, 1)
	appearanceGrid.Attach(fontEntry, 1, 0, 1, 1)

	sizeLabel, _ := gtk.LabelNew("Font Size:")
	sizeLabel.SetHAlign(gtk.ALIGN_END)
	sizeAdj, _ := gtk.AdjustmentNew(float64(a.config.FontSize), 6, 72, 1, 4, 0)
	sizeSpin, _ := gtk.SpinButtonNew(sizeAdj, 1, 0)
	appearanceGrid.Attach(sizeLabel, 0, 1, 1, 1)
	appearanceGrid.Attach(sizeSpin, 1, 1, 1, 1)

	fgLabel, _ := gtk.LabelNew("Foreground:")
	fgLabel.SetHAlign(gtk.ALIGN_END)
	fgBtn, _ := gtk.ColorButtonNew()
	fgBtn.SetTitle("Foreground Color")
	appearanceGrid.Attach(fgLabel, 0, 2, 1, 1)
	appearanceGrid.Attach(fgBtn, 1, 2, 1, 1)

	bgLabel, _ := gtk.LabelNew("Background:")
	bgLabel.SetHAlign(gtk.ALIGN_END)
	bgBtn, _ := gtk.ColorButtonNew()
	bgBtn.SetTitle("Background Color")
	appearanceGrid.Attach(bgLabel, 0, 3, 1, 1)
	appearanceGrid.Attach(bgBtn, 1, 3, 1, 1)

	appearanceLabel, _ := gtk.LabelNew("Appearance")
	notebook.AppendPage(appearanceGrid, appearanceLabel)

	terminalGrid, _ := gtk.GridNew()
	terminalGrid.SetColumnSpacing(12)
	terminalGrid.SetRowSpacing(12)
	terminalGrid.SetMarginStart(12)
	terminalGrid.SetMarginEnd(12)
	terminalGrid.SetMarginTop(12)

	scrollLabel, _ := gtk.LabelNew("Scrollback Lines:")
	scrollLabel.SetHAlign(gtk.ALIGN_END)
	scrollAdj, _ := gtk.AdjustmentNew(float64(a.config.Scrollback), 100, 1000000, 100, 1000, 0)
	scrollSpin, _ := gtk.SpinButtonNew(scrollAdj, 100, 0)
	terminalGrid.Attach(scrollLabel, 0, 0, 1, 1)
	terminalGrid.Attach(scrollSpin, 1, 0, 1, 1)

	terminalLabel, _ := gtk.LabelNew("Terminal")
	notebook.AppendPage(terminalGrid, terminalLabel)

	contentArea.Add(notebook)
	dialog.ShowAll()

	response := dialog.Run()
	if response == gtk.RESPONSE_OK {
		fontText, _ := fontEntry.GetText()
		a.config.Font = fontText
		a.config.FontSize = int(sizeSpin.GetValue())
		a.config.Scrollback = int64(scrollSpin.GetValue())

		fgRGBA := fgBtn.GetRGBA()
		a.config.Foreground = fmt.Sprintf("#%02X%02X%02X",
			int(fgRGBA.GetRed()*255),
			int(fgRGBA.GetGreen()*255),
			int(fgRGBA.GetBlue()*255))

		bgRGBA := bgBtn.GetRGBA()
		a.config.Background = fmt.Sprintf("#%02X%02X%02X",
			int(bgRGBA.GetRed()*255),
			int(bgRGBA.GetGreen()*255),
			int(bgRGBA.GetBlue()*255))

		a.saveConfig()
		a.applyTerminalSettings()
	}
	dialog.Destroy()
}
