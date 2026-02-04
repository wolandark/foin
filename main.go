package main

/*
#cgo pkg-config: gtk+-3.0 vte-2.91
#include <gtk/gtk.h>
#include <vte/vte.h>
#include <stdlib.h>

extern gboolean goButtonReleaseCallback(GdkEventButton *event);
extern gboolean goKeyPressCallback(GdkEventKey *event);

static gboolean on_button_release(GtkWidget *widget, GdkEventButton *event, gpointer user_data) {
    return goButtonReleaseCallback(event);
}

static gboolean on_key_press(GtkWidget *widget, GdkEventKey *event, gpointer user_data) {
    return goKeyPressCallback(event);
}

static GtkWidget* create_vte_terminal() {
    GtkWidget *term = vte_terminal_new();
    g_signal_connect(term, "button-release-event", G_CALLBACK(on_button_release), NULL);
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

static void vte_set_colors_full(VteTerminal *terminal, const char *fg, const char *bg, const char *cursor, const char **palette, int palette_size) {
    GdkRGBA foreground, background, cursor_color;
    GdkRGBA colors[16];
    
    gdk_rgba_parse(&foreground, fg);
    gdk_rgba_parse(&background, bg);
    
    if (cursor != NULL && strlen(cursor) > 0) {
        gdk_rgba_parse(&cursor_color, cursor);
        vte_terminal_set_color_cursor(terminal, &cursor_color);
    }
    
    if (palette != NULL && palette_size == 16) {
        for (int i = 0; i < 16; i++) {
            gdk_rgba_parse(&colors[i], palette[i]);
        }
        vte_terminal_set_colors(terminal, &foreground, &background, colors, 16);
    } else {
        vte_terminal_set_colors(terminal, &foreground, &background, NULL, 0);
    }
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

static void vte_set_cursor_shape(VteTerminal *terminal, int shape) {
    vte_terminal_set_cursor_shape(terminal, (VteCursorShape)shape);
}

static void vte_set_cursor_blink(VteTerminal *terminal, int mode) {
    vte_terminal_set_cursor_blink_mode(terminal, (VteCursorBlinkMode)mode);
}

static void vte_set_audible_bell(VteTerminal *terminal, gboolean enable) {
    vte_terminal_set_audible_bell(terminal, enable);
}

static void vte_set_allow_hyperlink(VteTerminal *terminal, gboolean enable) {
    vte_terminal_set_allow_hyperlink(terminal, enable);
}

static void vte_set_bold_is_bright(VteTerminal *terminal, gboolean enable) {
    vte_terminal_set_bold_is_bright(terminal, enable);
}

static void vte_set_scroll_on_output(VteTerminal *terminal, gboolean enable) {
    vte_terminal_set_scroll_on_output(terminal, enable);
}

static void vte_set_scroll_on_keystroke(VteTerminal *terminal, gboolean enable) {
    vte_terminal_set_scroll_on_keystroke(terminal, enable);
}

static void vte_set_mouse_autohide(VteTerminal *terminal, gboolean enable) {
    vte_terminal_set_mouse_autohide(terminal, enable);
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
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

type ColorPreset struct {
	Name        string
	Foreground  string
	Background  string
	CursorColor string
	Palette     []string
}

var colorPresets = []ColorPreset{
	{
		Name:        "Catppuccin Mocha",
		Foreground:  "#CDD6F4",
		Background:  "#1E1E2E",
		CursorColor: "#F5E0DC",
		Palette: []string{
			"#45475A", "#F38BA8", "#A6E3A1", "#F9E2AF",
			"#89B4FA", "#F5C2E7", "#94E2D5", "#BAC2DE",
			"#585B70", "#F38BA8", "#A6E3A1", "#F9E2AF",
			"#89B4FA", "#F5C2E7", "#94E2D5", "#A6ADC8",
		},
	},
	{
		Name:        "Catppuccin Latte",
		Foreground:  "#4C4F69",
		Background:  "#EFF1F5",
		CursorColor: "#DC8A78",
		Palette: []string{
			"#5C5F77", "#D20F39", "#40A02B", "#DF8E1D",
			"#1E66F5", "#EA76CB", "#179299", "#ACB0BE",
			"#6C6F85", "#D20F39", "#40A02B", "#DF8E1D",
			"#1E66F5", "#EA76CB", "#179299", "#BCC0CC",
		},
	},
	{
		Name:        "GNOME Terminal",
		Foreground:  "#D3D7CF",
		Background:  "#2E3436",
		CursorColor: "#D3D7CF",
		Palette: []string{
			"#2E3436", "#CC0000", "#4E9A06", "#C4A000",
			"#3465A4", "#75507B", "#06989A", "#D3D7CF",
			"#555753", "#EF2929", "#8AE234", "#FCE94F",
			"#729FCF", "#AD7FA8", "#34E2E2", "#EEEEEC",
		},
	},
	{
		Name:        "XTerm",
		Foreground:  "#FFFFFF",
		Background:  "#000000",
		CursorColor: "#FFFFFF",
		Palette: []string{
			"#000000", "#CD0000", "#00CD00", "#CDCD00",
			"#0000EE", "#CD00CD", "#00CDCD", "#E5E5E5",
			"#7F7F7F", "#FF0000", "#00FF00", "#FFFF00",
			"#5C5CFF", "#FF00FF", "#00FFFF", "#FFFFFF",
		},
	},
	{
		Name:        "Solarized Dark",
		Foreground:  "#839496",
		Background:  "#002B36",
		CursorColor: "#839496",
		Palette: []string{
			"#073642", "#DC322F", "#859900", "#B58900",
			"#268BD2", "#D33682", "#2AA198", "#EEE8D5",
			"#002B36", "#CB4B16", "#586E75", "#657B83",
			"#839496", "#6C71C4", "#93A1A1", "#FDF6E3",
		},
	},
	{
		Name:        "Solarized Light",
		Foreground:  "#657B83",
		Background:  "#FDF6E3",
		CursorColor: "#657B83",
		Palette: []string{
			"#073642", "#DC322F", "#859900", "#B58900",
			"#268BD2", "#D33682", "#2AA198", "#EEE8D5",
			"#002B36", "#CB4B16", "#586E75", "#657B83",
			"#839496", "#6C71C4", "#93A1A1", "#FDF6E3",
		},
	},
	{
		Name:        "Dracula",
		Foreground:  "#F8F8F2",
		Background:  "#282A36",
		CursorColor: "#F8F8F2",
		Palette: []string{
			"#21222C", "#FF5555", "#50FA7B", "#F1FA8C",
			"#BD93F9", "#FF79C6", "#8BE9FD", "#F8F8F2",
			"#6272A4", "#FF6E6E", "#69FF94", "#FFFFA5",
			"#D6ACFF", "#FF92DF", "#A4FFFF", "#FFFFFF",
		},
	},
	{
		Name:        "Nord",
		Foreground:  "#D8DEE9",
		Background:  "#2E3440",
		CursorColor: "#D8DEE9",
		Palette: []string{
			"#3B4252", "#BF616A", "#A3BE8C", "#EBCB8B",
			"#81A1C1", "#B48EAD", "#88C0D0", "#E5E9F0",
			"#4C566A", "#BF616A", "#A3BE8C", "#EBCB8B",
			"#81A1C1", "#B48EAD", "#8FBCBB", "#ECEFF4",
		},
	},
	{
		Name:        "Gruvbox Dark",
		Foreground:  "#EBDBB2",
		Background:  "#282828",
		CursorColor: "#EBDBB2",
		Palette: []string{
			"#282828", "#CC241D", "#98971A", "#D79921",
			"#458588", "#B16286", "#689D6A", "#A89984",
			"#928374", "#FB4934", "#B8BB26", "#FABD2F",
			"#83A598", "#D3869B", "#8EC07C", "#EBDBB2",
		},
	},
	{
		Name:        "Tokyo Night",
		Foreground:  "#A9B1D6",
		Background:  "#1A1B26",
		CursorColor: "#C0CAF5",
		Palette: []string{
			"#15161E", "#F7768E", "#9ECE6A", "#E0AF68",
			"#7AA2F7", "#BB9AF7", "#7DCFFF", "#A9B1D6",
			"#414868", "#F7768E", "#9ECE6A", "#E0AF68",
			"#7AA2F7", "#BB9AF7", "#7DCFFF", "#C0CAF5",
		},
	},
	{
		Name:        "One Dark",
		Foreground:  "#ABB2BF",
		Background:  "#282C34",
		CursorColor: "#528BFF",
		Palette: []string{
			"#282C34", "#E06C75", "#98C379", "#E5C07B",
			"#61AFEF", "#C678DD", "#56B6C2", "#ABB2BF",
			"#545862", "#E06C75", "#98C379", "#E5C07B",
			"#61AFEF", "#C678DD", "#56B6C2", "#C8CCD4",
		},
	},
	{
		Name:        "Matrix",
		Foreground:  "#00FF00",
		Background:  "#000000",
		CursorColor: "#00FF00",
		Palette: []string{
			"#000000", "#008000", "#00B800", "#00D800",
			"#003000", "#005000", "#007000", "#00FF00",
			"#003000", "#00A000", "#00C000", "#00E000",
			"#004000", "#006000", "#008000", "#00FF00",
		},
	},
	{
		Name:        "Pastel",
		Foreground:  "#4A4A4A",
		Background:  "#F5F5F5",
		CursorColor: "#4A4A4A",
		Palette: []string{
			"#4A4A4A", "#E57373", "#81C784", "#FFD54F",
			"#64B5F6", "#BA68C8", "#4DD0E1", "#F5F5F5",
			"#757575", "#EF9A9A", "#A5D6A7", "#FFE082",
			"#90CAF9", "#CE93D8", "#80DEEA", "#FFFFFF",
		},
	},
	{
		Name:        "White on Black",
		Foreground:  "#FFFFFF",
		Background:  "#000000",
		CursorColor: "#FFFFFF",
		Palette: []string{
			"#000000", "#AA0000", "#00AA00", "#AA5500",
			"#0000AA", "#AA00AA", "#00AAAA", "#AAAAAA",
			"#555555", "#FF5555", "#55FF55", "#FFFF55",
			"#5555FF", "#FF55FF", "#55FFFF", "#FFFFFF",
		},
	},
	{
		Name:        "Black on White",
		Foreground:  "#000000",
		Background:  "#FFFFFF",
		CursorColor: "#000000",
		Palette: []string{
			"#000000", "#AA0000", "#00AA00", "#AA5500",
			"#0000AA", "#AA00AA", "#00AAAA", "#AAAAAA",
			"#555555", "#FF5555", "#55FF55", "#FFFF55",
			"#5555FF", "#FF55FF", "#55FFFF", "#FFFFFF",
		},
	},
	{
		Name:        "Monokai",
		Foreground:  "#F8F8F2",
		Background:  "#272822",
		CursorColor: "#F8F8F2",
		Palette: []string{
			"#272822", "#F92672", "#A6E22E", "#F4BF75",
			"#66D9EF", "#AE81FF", "#A1EFE4", "#F8F8F2",
			"#75715E", "#F92672", "#A6E22E", "#F4BF75",
			"#66D9EF", "#AE81FF", "#A1EFE4", "#F9F8F5",
		},
	},
	{
		Name:        "Cyberpunk",
		Foreground:  "#00FFFF",
		Background:  "#0D0221",
		CursorColor: "#FF00FF",
		Palette: []string{
			"#0D0221", "#FF2A6D", "#05D9E8", "#D1F7FF",
			"#7B00FF", "#FF00FF", "#01C4E7", "#00FFFF",
			"#1A1A2E", "#FF6B9D", "#65FCEB", "#F5FEFD",
			"#9D4EDD", "#FF69EB", "#01E5F7", "#FFFFFF",
		},
	},
}

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
	Font              string   `json:"font"`
	FontSize          int      `json:"font_size"`
	Foreground        string   `json:"foreground"`
	Background        string   `json:"background"`
	CursorColor       string   `json:"cursor_color"`
	Palette           []string `json:"palette"`
	Scrollback        int64    `json:"scrollback"`
	CursorShape       int      `json:"cursor_shape"`
	CursorBlink       int      `json:"cursor_blink"`
	AudibleBell       bool     `json:"audible_bell"`
	VisualBell        bool     `json:"visual_bell"`
	AllowHyperlinks   bool     `json:"allow_hyperlinks"`
	BoldIsBright      bool     `json:"bold_is_bright"`
	ScrollOnOutput    bool     `json:"scroll_on_output"`
	ScrollOnKeystroke bool     `json:"scroll_on_keystroke"`
	MouseAutohide     bool     `json:"mouse_autohide"`
}

type TerminalTab struct {
	widget   *C.GtkWidget
	vte      *C.VteTerminal
	shellPid int
}

type App struct {
	window        *gtk.Window
	notebook      *gtk.Notebook
	sidebar       *gtk.Box
	sidebarBtn    *gtk.ToggleButton
	tabs          []*TerminalTab
	activeTab     int
	hostList      *gtk.ListBox
	hosts         []SSHHost
	config        Config
	configPath    string
	hostsPath     string
	mu            sync.Mutex
	seenSSHPids   map[int]bool
	sidebarVisible bool
}

var app *App

func main() {
	gtk.Init(nil)

	app = &App{
		seenSSHPids: make(map[int]bool),
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
		Font:       "JetBrains Mono",
		FontSize:   11,
		Foreground: "#CDD6F4",
		Background: "#1E1E2E",
		CursorColor: "#F5E0DC",
		Palette: []string{
			"#45475A", "#F38BA8", "#A6E3A1", "#F9E2AF",
			"#89B4FA", "#F5C2E7", "#94E2D5", "#BAC2DE",
			"#585B70", "#F38BA8", "#A6E3A1", "#F9E2AF",
			"#89B4FA", "#F5C2E7", "#94E2D5", "#A6ADC8",
		},
		Scrollback:        10000,
		CursorShape:       0,
		CursorBlink:       0,
		AudibleBell:       false,
		VisualBell:        false,
		AllowHyperlinks:   true,
		BoldIsBright:      true,
		ScrollOnOutput:    false,
		ScrollOnKeystroke: true,
		MouseAutohide:     false,
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

	a.sidebar, _ = gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	a.sidebar.SetSizeRequest(220, -1)
	a.sidebarVisible = true

	sidebarStyle, _ := a.sidebar.GetStyleContext()
	cssProvider, _ := gtk.CssProviderNew()
	cssProvider.LoadFromData(`
		.sidebar {
			background-color: #181825;
			border-right: 1px solid #313244;
		}
		.sidebar-header {
			background-color: #1E1E2E;
			padding: 8px 12px;
			border-bottom: 1px solid #313244;
		}
		.sidebar-header label {
			color: #CDD6F4;
			font-weight: bold;
			font-size: 13px;
		}
		.add-button {
			background: transparent;
			border: none;
			color: #94E2D5;
			min-width: 28px;
			min-height: 28px;
			padding: 0;
		}
		.add-button:hover {
			background-color: #313244;
		}
		.host-row {
			padding: 8px 12px;
			border-bottom: 1px solid #181825;
		}
		.host-row:hover {
			background-color: #313244;
		}
		.host-row:selected {
			background-color: #45475A;
		}
		.host-name {
			color: #89B4FA;
			font-weight: 500;
		}
		.host-detail {
			color: #6C7086;
			font-size: 11px;
		}
		.delete-btn {
			background: transparent;
			border: none;
			color: #F38BA8;
			min-width: 20px;
			min-height: 20px;
			padding: 2px;
			opacity: 0.6;
		}
		.delete-btn:hover {
			opacity: 1;
			background-color: rgba(243, 139, 168, 0.2);
		}
		notebook > header {
			background-color: #1E1E2E;
		}
		notebook > header tab {
			background-color: #181825;
			color: #BAC2DE;
			padding: 4px 8px;
			border: none;
		}
		notebook > header tab:checked {
			background-color: #313244;
			color: #CDD6F4;
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

	a.sidebar.PackStart(headerBox, false, false, 0)

	scrolled, _ := gtk.ScrolledWindowNew(nil, nil)
	scrolled.SetPolicy(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC)

	a.hostList, _ = gtk.ListBoxNew()
	a.hostList.SetSelectionMode(gtk.SELECTION_SINGLE)
	a.hostList.Connect("row-activated", a.onHostActivated)
	scrolled.Add(a.hostList)

	a.sidebar.PackStart(scrolled, true, true, 0)
	mainBox.PackStart(a.sidebar, false, false, 0)

	termBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)

	a.notebook, _ = gtk.NotebookNew()
	a.notebook.SetScrollable(true)
	a.notebook.SetShowBorder(false)
	a.notebook.Connect("switch-page", func(nb *gtk.Notebook, page *gtk.Widget, pageNum uint) {
		a.activeTab = int(pageNum)
	})

	a.addNewTab()

	actionBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 4)

	a.sidebarBtn, _ = gtk.ToggleButtonNew()
	sidebarIcon, _ := gtk.ImageNewFromIconName("view-sidebar-symbolic", gtk.ICON_SIZE_MENU)
	a.sidebarBtn.SetImage(sidebarIcon)
	a.sidebarBtn.SetRelief(gtk.RELIEF_NONE)
	a.sidebarBtn.SetActive(true)
	a.sidebarBtn.SetTooltipText("Toggle SSH Panel (Ctrl+Shift+B)")
	a.sidebarBtn.Connect("toggled", func() {
		a.toggleSidebar()
	})
	actionBox.PackStart(a.sidebarBtn, false, false, 0)

	newTabBtn, _ := gtk.ButtonNewFromIconName("tab-new-symbolic", gtk.ICON_SIZE_MENU)
	newTabBtn.SetRelief(gtk.RELIEF_NONE)
	newTabBtn.SetTooltipText("New Tab (Ctrl+Shift+T)")
	newTabBtn.Connect("clicked", func() {
		a.addNewTab()
	})
	actionBox.PackStart(newTabBtn, false, false, 0)
	actionBox.ShowAll()

	a.notebook.SetActionWidget(actionBox, gtk.PACK_END)

	termBox.PackStart(a.notebook, true, true, 0)
	mainBox.PackStart(termBox, true, true, 0)

	a.window.Add(mainBox)
	a.refreshHostList()

	return nil
}

func (a *App) addNewTab() {
	terminal := C.create_vte_terminal()
	vte := (*C.VteTerminal)(unsafe.Pointer(terminal))

	tab := &TerminalTab{
		widget: terminal,
		vte:    vte,
	}
	a.tabs = append(a.tabs, tab)

	termWidget := glib.Take(unsafe.Pointer(terminal))
	termGtk := &gtk.Widget{glib.InitiallyUnowned{termWidget}}
	termGtk.SetHExpand(true)
	termGtk.SetVExpand(true)

	a.applySettingsToTerminal(vte)

	homeDir, _ := os.UserHomeDir()
	cHomeDir := C.CString(homeDir)
	C.vte_spawn_shell(vte, cHomeDir)
	C.free(unsafe.Pointer(cHomeDir))

	tabNum := len(a.tabs)
	labelBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 4)
	label, _ := gtk.LabelNew(fmt.Sprintf("Terminal %d", tabNum))
	closeBtn, _ := gtk.ButtonNewFromIconName("window-close-symbolic", gtk.ICON_SIZE_MENU)
	closeBtn.SetRelief(gtk.RELIEF_NONE)

	tabIndex := tabNum - 1
	closeBtn.Connect("clicked", func() {
		a.closeTab(tabIndex)
	})

	labelBox.PackStart(label, true, true, 0)
	labelBox.PackEnd(closeBtn, false, false, 0)
	labelBox.ShowAll()

	pageNum := a.notebook.AppendPage(termGtk, labelBox)
	a.notebook.ShowAll()
	a.notebook.SetCurrentPage(pageNum)

	go a.findShellPidForTab(tab)
}

func (a *App) closeTab(index int) {
	if len(a.tabs) <= 1 {
		return
	}

	a.notebook.RemovePage(index)

	a.mu.Lock()
	if index < len(a.tabs) {
		a.tabs = append(a.tabs[:index], a.tabs[index+1:]...)
	}
	a.mu.Unlock()
}

func (a *App) getCurrentTab() *TerminalTab {
	if a.activeTab >= 0 && a.activeTab < len(a.tabs) {
		return a.tabs[a.activeTab]
	}
	if len(a.tabs) > 0 {
		return a.tabs[0]
	}
	return nil
}

func (a *App) toggleSidebar() {
	a.sidebarVisible = !a.sidebarVisible
	if a.sidebarVisible {
		a.sidebar.Show()
	} else {
		a.sidebar.Hide()
	}
	a.sidebarBtn.SetActive(a.sidebarVisible)
}

func (a *App) applyTerminalSettings() {
	for _, tab := range a.tabs {
		a.applySettingsToTerminal(tab.vte)
	}
}

func (a *App) applySettingsToTerminal(vte *C.VteTerminal) {
	fontDesc := fmt.Sprintf("%s %d", a.config.Font, a.config.FontSize)
	cFont := C.CString(fontDesc)
	defer C.free(unsafe.Pointer(cFont))
	C.vte_set_font_from_string(vte, cFont)

	cFg := C.CString(a.config.Foreground)
	cBg := C.CString(a.config.Background)
	cCursor := C.CString(a.config.CursorColor)
	defer C.free(unsafe.Pointer(cFg))
	defer C.free(unsafe.Pointer(cBg))
	defer C.free(unsafe.Pointer(cCursor))

	if len(a.config.Palette) == 16 {
		cPalette := make([]*C.char, 16)
		for i, color := range a.config.Palette {
			cPalette[i] = C.CString(color)
			defer C.free(unsafe.Pointer(cPalette[i]))
		}
		C.vte_set_colors_full(vte, cFg, cBg, cCursor, &cPalette[0], 16)
	} else {
		C.vte_set_colors_full(vte, cFg, cBg, cCursor, nil, 0)
	}

	C.vte_set_scrollback(vte, C.glong(a.config.Scrollback))
	C.vte_set_cursor_shape(vte, C.int(a.config.CursorShape))
	C.vte_set_cursor_blink(vte, C.int(a.config.CursorBlink))
	C.vte_set_audible_bell(vte, boolToGboolean(a.config.AudibleBell))
	C.vte_set_allow_hyperlink(vte, boolToGboolean(a.config.AllowHyperlinks))
	C.vte_set_bold_is_bright(vte, boolToGboolean(a.config.BoldIsBright))
	C.vte_set_scroll_on_output(vte, boolToGboolean(a.config.ScrollOnOutput))
	C.vte_set_scroll_on_keystroke(vte, boolToGboolean(a.config.ScrollOnKeystroke))
	C.vte_set_mouse_autohide(vte, boolToGboolean(a.config.MouseAutohide))
}

func boolToGboolean(b bool) C.gboolean {
	if b {
		return C.TRUE
	}
	return C.FALSE
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

	tab := a.getCurrentTab()
	if tab == nil {
		return
	}

	cmdStr := cmd.String()
	cCmd := C.CString(cmdStr)
	defer C.free(unsafe.Pointer(cCmd))
	C.vte_feed_child_text(tab.vte, cCmd)
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

//export goButtonReleaseCallback
func goButtonReleaseCallback(event *C.GdkEventButton) C.gboolean {
	button := C.get_event_button(event)
	if button == 3 {
		app.showContextMenuAt()
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
		tab := app.getCurrentTab()
		if tab == nil {
			return C.FALSE
		}
		switch keyval {
		case C.guint(gdk.KEY_C):
			C.vte_copy_clipboard(tab.vte)
			return C.TRUE
		case C.guint(gdk.KEY_V):
			C.vte_paste_clipboard(tab.vte)
			return C.TRUE
		case C.guint(gdk.KEY_T):
			glib.IdleAdd(func() {
				app.addNewTab()
			})
			return C.TRUE
		case C.guint(gdk.KEY_B):
			glib.IdleAdd(func() {
				app.toggleSidebar()
			})
			return C.TRUE
		case C.guint(gdk.KEY_W):
			glib.IdleAdd(func() {
				app.closeTab(app.activeTab)
			})
			return C.TRUE
		}
	}

	return C.FALSE
}


func (a *App) startSSHMonitor() {
	go func() {
		for {
			time.Sleep(2 * time.Second)
			a.scanForSSHProcesses()
		}
	}()
}

func (a *App) scanForSSHProcesses() {
	var allChildPids []int

	a.mu.Lock()
	for _, tab := range a.tabs {
		if tab.shellPid != 0 {
			allChildPids = append(allChildPids, a.getChildPids(tab.shellPid)...)
		}
	}
	a.mu.Unlock()

	for _, pid := range allChildPids {
		a.mu.Lock()
		seen := a.seenSSHPids[pid]
		a.mu.Unlock()

		if seen {
			continue
		}

		cmdline := a.getProcessCmdline(pid)
		if cmdline == "" {
			continue
		}

		parts := strings.Split(cmdline, "\x00")
		if len(parts) == 0 {
			continue
		}

		exe := filepath.Base(parts[0])
		if exe != "ssh" {
			continue
		}

		a.mu.Lock()
		a.seenSSHPids[pid] = true
		a.mu.Unlock()

		host, sshUser, port := a.parseSSHArgs(parts)
		if host == "" {
			continue
		}

		a.addSSHHost(host, sshUser, port)
	}
}

func (a *App) getChildPids(parentPid int) []int {
	var children []int

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return children
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		statPath := fmt.Sprintf("/proc/%d/stat", pid)
		data, err := os.ReadFile(statPath)
		if err != nil {
			continue
		}

		fields := strings.Fields(string(data))
		if len(fields) < 4 {
			continue
		}

		ppid, err := strconv.Atoi(fields[3])
		if err != nil {
			continue
		}

		if ppid == parentPid {
			children = append(children, pid)
			children = append(children, a.getChildPids(pid)...)
		}
	}

	return children
}

func (a *App) getProcessCmdline(pid int) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return ""
	}
	return string(data)
}

func (a *App) parseSSHArgs(args []string) (host, sshUser, port string) {
	port = "22"
	currentUser, _ := user.Current()
	sshUser = currentUser.Username

	skipNext := false
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if arg == "" {
			continue
		}

		if skipNext {
			skipNext = false
			continue
		}

		if strings.HasPrefix(arg, "-") {
			switch arg {
			case "-p":
				if i+1 < len(args) {
					port = args[i+1]
					skipNext = true
				}
			case "-l":
				if i+1 < len(args) {
					sshUser = args[i+1]
					skipNext = true
				}
			case "-i", "-o", "-F", "-J", "-L", "-R", "-D", "-W", "-b", "-c", "-e", "-m", "-O", "-Q", "-S", "-w", "-E", "-B", "-I":
				skipNext = true
			}
			continue
		}

		if strings.Contains(arg, "@") {
			parts := strings.SplitN(arg, "@", 2)
			sshUser = parts[0]
			host = parts[1]
		} else if host == "" {
			host = arg
		}
	}

	if strings.Contains(host, ":") {
		parts := strings.SplitN(host, ":", 2)
		host = parts[0]
	}

	return host, sshUser, port
}

func (a *App) addSSHHost(host, sshUser, port string) {
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

func (a *App) findShellPidForTab(tab *TerminalTab) {
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)

		entries, err := os.ReadDir("/proc")
		if err != nil {
			continue
		}

		myPid := os.Getpid()

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			pid, err := strconv.Atoi(entry.Name())
			if err != nil {
				continue
			}

			statPath := fmt.Sprintf("/proc/%d/stat", pid)
			data, err := os.ReadFile(statPath)
			if err != nil {
				continue
			}

			fields := strings.Fields(string(data))
			if len(fields) < 4 {
				continue
			}

			ppid, err := strconv.Atoi(fields[3])
			if err != nil {
				continue
			}

			if ppid == myPid {
				cmdline := a.getProcessCmdline(pid)
				if strings.Contains(cmdline, "sh") || strings.Contains(cmdline, "bash") || strings.Contains(cmdline, "zsh") || strings.Contains(cmdline, "fish") {
					tab.shellPid = pid
					a.startSSHMonitor()
					return
				}
			}
		}
	}
}

func (a *App) showContextMenuAt() {
	tab := a.getCurrentTab()
	if tab == nil {
		return
	}

	menu, _ := gtk.MenuNew()

	copyItem, _ := gtk.MenuItemNewWithLabel("Copy")
	copyItem.Connect("activate", func() {
		if t := a.getCurrentTab(); t != nil {
			C.vte_copy_clipboard(t.vte)
		}
	})
	menu.Append(copyItem)

	pasteItem, _ := gtk.MenuItemNewWithLabel("Paste")
	pasteItem.Connect("activate", func() {
		if t := a.getCurrentTab(); t != nil {
			C.vte_paste_clipboard(t.vte)
		}
	})
	menu.Append(pasteItem)

	selectAllItem, _ := gtk.MenuItemNewWithLabel("Select All")
	selectAllItem.Connect("activate", func() {
		if t := a.getCurrentTab(); t != nil {
			C.vte_select_all(t.vte)
		}
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
	dialog.SetDefaultSize(550, 550)

	contentArea, _ := dialog.GetContentArea()
	contentArea.SetMarginStart(20)
	contentArea.SetMarginEnd(20)
	contentArea.SetMarginTop(20)
	contentArea.SetMarginBottom(10)
	contentArea.SetSpacing(16)

	notebook, _ := gtk.NotebookNew()

	appearanceGrid, _ := gtk.GridNew()
	appearanceGrid.SetColumnSpacing(12)
	appearanceGrid.SetRowSpacing(10)
	appearanceGrid.SetMarginStart(12)
	appearanceGrid.SetMarginEnd(12)
	appearanceGrid.SetMarginTop(12)

	row := 0

	fontLabel, _ := gtk.LabelNew("Font:")
	fontLabel.SetHAlign(gtk.ALIGN_END)
	fontBtn, _ := gtk.FontButtonNew()
	fontBtn.SetFont(fmt.Sprintf("%s %d", a.config.Font, a.config.FontSize))
	fontBtn.SetHExpand(true)
	appearanceGrid.Attach(fontLabel, 0, row, 1, 1)
	appearanceGrid.Attach(fontBtn, 1, row, 1, 1)
	row++

	presetLabel, _ := gtk.LabelNew("Color Preset:")
	presetLabel.SetHAlign(gtk.ALIGN_END)
	presetCombo, _ := gtk.ComboBoxTextNew()
	presetCombo.AppendText("Custom")
	for _, preset := range colorPresets {
		presetCombo.AppendText(preset.Name)
	}
	presetCombo.SetActive(0)
	appearanceGrid.Attach(presetLabel, 0, row, 1, 1)
	appearanceGrid.Attach(presetCombo, 1, row, 1, 1)
	row++

	fgLabel, _ := gtk.LabelNew("Foreground:")
	fgLabel.SetHAlign(gtk.ALIGN_END)
	fgBtn, _ := gtk.ColorButtonNew()
	fgBtn.SetTitle("Foreground Color")
	a.setColorButtonFromHex(fgBtn, a.config.Foreground)
	appearanceGrid.Attach(fgLabel, 0, row, 1, 1)
	appearanceGrid.Attach(fgBtn, 1, row, 1, 1)
	row++

	bgLabel, _ := gtk.LabelNew("Background:")
	bgLabel.SetHAlign(gtk.ALIGN_END)
	bgBtn, _ := gtk.ColorButtonNew()
	bgBtn.SetTitle("Background Color")
	a.setColorButtonFromHex(bgBtn, a.config.Background)
	appearanceGrid.Attach(bgLabel, 0, row, 1, 1)
	appearanceGrid.Attach(bgBtn, 1, row, 1, 1)
	row++

	cursorColorLabel, _ := gtk.LabelNew("Cursor Color:")
	cursorColorLabel.SetHAlign(gtk.ALIGN_END)
	cursorColorBtn, _ := gtk.ColorButtonNew()
	cursorColorBtn.SetTitle("Cursor Color")
	a.setColorButtonFromHex(cursorColorBtn, a.config.CursorColor)
	appearanceGrid.Attach(cursorColorLabel, 0, row, 1, 1)
	appearanceGrid.Attach(cursorColorBtn, 1, row, 1, 1)
	row++

	cursorShapeLabel, _ := gtk.LabelNew("Cursor Shape:")
	cursorShapeLabel.SetHAlign(gtk.ALIGN_END)
	cursorShapeCombo, _ := gtk.ComboBoxTextNew()
	cursorShapeCombo.AppendText("Block")
	cursorShapeCombo.AppendText("I-Beam")
	cursorShapeCombo.AppendText("Underline")
	cursorShapeCombo.SetActive(a.config.CursorShape)
	appearanceGrid.Attach(cursorShapeLabel, 0, row, 1, 1)
	appearanceGrid.Attach(cursorShapeCombo, 1, row, 1, 1)
	row++

	cursorBlinkLabel, _ := gtk.LabelNew("Cursor Blink:")
	cursorBlinkLabel.SetHAlign(gtk.ALIGN_END)
	cursorBlinkCombo, _ := gtk.ComboBoxTextNew()
	cursorBlinkCombo.AppendText("System Default")
	cursorBlinkCombo.AppendText("On")
	cursorBlinkCombo.AppendText("Off")
	cursorBlinkCombo.SetActive(a.config.CursorBlink)
	appearanceGrid.Attach(cursorBlinkLabel, 0, row, 1, 1)
	appearanceGrid.Attach(cursorBlinkCombo, 1, row, 1, 1)
	row++

	paletteLabel, _ := gtk.LabelNew("Color Palette:")
	paletteLabel.SetHAlign(gtk.ALIGN_END)
	paletteLabel.SetVAlign(gtk.ALIGN_START)
	paletteGrid, _ := gtk.GridNew()
	paletteGrid.SetColumnSpacing(4)
	paletteGrid.SetRowSpacing(4)

	paletteButtons := make([]*gtk.ColorButton, 16)
	for i := 0; i < 16; i++ {
		btn, _ := gtk.ColorButtonNew()
		if i < len(a.config.Palette) {
			a.setColorButtonFromHex(btn, a.config.Palette[i])
		}
		paletteButtons[i] = btn
		paletteGrid.Attach(btn, i%8, i/8, 1, 1)
	}

	presetCombo.Connect("changed", func() {
		idx := presetCombo.GetActive()
		if idx > 0 && idx <= len(colorPresets) {
			preset := colorPresets[idx-1]
			a.setColorButtonFromHex(fgBtn, preset.Foreground)
			a.setColorButtonFromHex(bgBtn, preset.Background)
			a.setColorButtonFromHex(cursorColorBtn, preset.CursorColor)
			for i := 0; i < 16 && i < len(preset.Palette); i++ {
				a.setColorButtonFromHex(paletteButtons[i], preset.Palette[i])
			}
		}
	})
	appearanceGrid.Attach(paletteLabel, 0, row, 1, 1)
	appearanceGrid.Attach(paletteGrid, 1, row, 1, 1)

	appearanceLabel, _ := gtk.LabelNew("Appearance")
	notebook.AppendPage(appearanceGrid, appearanceLabel)

	terminalGrid, _ := gtk.GridNew()
	terminalGrid.SetColumnSpacing(12)
	terminalGrid.SetRowSpacing(10)
	terminalGrid.SetMarginStart(12)
	terminalGrid.SetMarginEnd(12)
	terminalGrid.SetMarginTop(12)

	row = 0

	scrollLabel, _ := gtk.LabelNew("Scrollback Lines:")
	scrollLabel.SetHAlign(gtk.ALIGN_END)
	scrollAdj, _ := gtk.AdjustmentNew(float64(a.config.Scrollback), 100, 1000000, 100, 1000, 0)
	scrollSpin, _ := gtk.SpinButtonNew(scrollAdj, 100, 0)
	terminalGrid.Attach(scrollLabel, 0, row, 1, 1)
	terminalGrid.Attach(scrollSpin, 1, row, 1, 1)
	row++

	scrollOnOutputCheck, _ := gtk.CheckButtonNewWithLabel("Scroll on output")
	scrollOnOutputCheck.SetActive(a.config.ScrollOnOutput)
	terminalGrid.Attach(scrollOnOutputCheck, 0, row, 2, 1)
	row++

	scrollOnKeystrokeCheck, _ := gtk.CheckButtonNewWithLabel("Scroll on keystroke")
	scrollOnKeystrokeCheck.SetActive(a.config.ScrollOnKeystroke)
	terminalGrid.Attach(scrollOnKeystrokeCheck, 0, row, 2, 1)
	row++

	audibleBellCheck, _ := gtk.CheckButtonNewWithLabel("Audible bell")
	audibleBellCheck.SetActive(a.config.AudibleBell)
	terminalGrid.Attach(audibleBellCheck, 0, row, 2, 1)
	row++

	visualBellCheck, _ := gtk.CheckButtonNewWithLabel("Visual bell")
	visualBellCheck.SetActive(a.config.VisualBell)
	terminalGrid.Attach(visualBellCheck, 0, row, 2, 1)
	row++

	allowHyperlinksCheck, _ := gtk.CheckButtonNewWithLabel("Allow hyperlinks")
	allowHyperlinksCheck.SetActive(a.config.AllowHyperlinks)
	terminalGrid.Attach(allowHyperlinksCheck, 0, row, 2, 1)
	row++

	boldIsBrightCheck, _ := gtk.CheckButtonNewWithLabel("Bold is bright")
	boldIsBrightCheck.SetActive(a.config.BoldIsBright)
	terminalGrid.Attach(boldIsBrightCheck, 0, row, 2, 1)
	row++

	mouseAutohideCheck, _ := gtk.CheckButtonNewWithLabel("Hide mouse cursor when typing")
	mouseAutohideCheck.SetActive(a.config.MouseAutohide)
	terminalGrid.Attach(mouseAutohideCheck, 0, row, 2, 1)

	terminalLabel, _ := gtk.LabelNew("Terminal")
	notebook.AppendPage(terminalGrid, terminalLabel)

	contentArea.Add(notebook)
	dialog.ShowAll()

	response := dialog.Run()
	if response == gtk.RESPONSE_OK {
		fontName := fontBtn.GetFont()
		a.parseFontName(fontName)
		a.config.Scrollback = int64(scrollSpin.GetValue())

		a.config.Foreground = a.getHexFromColorButton(fgBtn)
		a.config.Background = a.getHexFromColorButton(bgBtn)
		a.config.CursorColor = a.getHexFromColorButton(cursorColorBtn)

		a.config.Palette = make([]string, 16)
		for i := 0; i < 16; i++ {
			a.config.Palette[i] = a.getHexFromColorButton(paletteButtons[i])
		}

		a.config.CursorShape = cursorShapeCombo.GetActive()
		a.config.CursorBlink = cursorBlinkCombo.GetActive()
		a.config.ScrollOnOutput = scrollOnOutputCheck.GetActive()
		a.config.ScrollOnKeystroke = scrollOnKeystrokeCheck.GetActive()
		a.config.AudibleBell = audibleBellCheck.GetActive()
		a.config.VisualBell = visualBellCheck.GetActive()
		a.config.AllowHyperlinks = allowHyperlinksCheck.GetActive()
		a.config.BoldIsBright = boldIsBrightCheck.GetActive()
		a.config.MouseAutohide = mouseAutohideCheck.GetActive()

		a.saveConfig()
		a.applyTerminalSettings()
	}
	dialog.Destroy()
}

func (a *App) setColorButtonFromHex(btn *gtk.ColorButton, hex string) {
	rgba := gdk.NewRGBA(0, 0, 0, 1)
	rgba.Parse(hex)
	btn.SetRGBA(rgba)
}

func (a *App) getHexFromColorButton(btn *gtk.ColorButton) string {
	rgba := btn.GetRGBA()
	return fmt.Sprintf("#%02X%02X%02X",
		int(rgba.GetRed()*255),
		int(rgba.GetGreen()*255),
		int(rgba.GetBlue()*255))
}

func (a *App) parseFontName(fontName string) {
	parts := strings.Split(fontName, " ")
	if len(parts) >= 2 {
		sizeStr := parts[len(parts)-1]
		size, err := strconv.Atoi(sizeStr)
		if err == nil {
			a.config.FontSize = size
			a.config.Font = strings.Join(parts[:len(parts)-1], " ")
			return
		}
	}
	a.config.Font = fontName
}
