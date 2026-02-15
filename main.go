package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	docStyle   = lipgloss.NewStyle().Margin(1, 2)
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00ADD8")).
			Bold(true).
			MarginLeft(2)
	warnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(lipgloss.Color("#FFA500")).
			Bold(true).
			Padding(0, 1).
			MarginLeft(2).
			MarginBottom(1)
)

type typeOption struct {
	label   string
	isAudio bool
}

func (t typeOption) Title() string       { return t.label }
func (t typeOption) Description() string {
	if !t.isAudio {
		return "Download video with sound"
	}
	return "Download " + t.label + " only"
}
func (t typeOption) FilterValue() string { return t.label }

type resolutionOption struct {
	label  string
	height int
}

func (r resolutionOption) Title() string       { return r.label }
func (r resolutionOption) Description() string { return fmt.Sprintf("Target height: %d", r.height) }
func (r resolutionOption) FilterValue() string { return r.label }

type fpsOption struct {
	id    string
	fps   int
	label string
	note  string
}

func (f fpsOption) Title() string       { return f.label }
func (f fpsOption) Description() string { return f.note }
func (f fpsOption) FilterValue() string { return f.label }

type downloadTask struct {
	url          string
	targetHeight int
	isAudioOnly  bool
}

type state int

const (
	stateInput state = iota
	stateSelectType
	stateSelectRes
	stateSelectFPS
)

type model struct {
	state        state
	textInput    textinput.Model
	list         list.Model
	choiceType   typeOption
	choiceRes    resolutionOption
	choiceFPS    fpsOption
	url          string
	quitting     bool
	errorMessage string
	width        int
	height       int
	listReady    bool
	notice       string
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "URL1, URL2, URL3 or path/to/links.txt"
	ti.Focus()
	ti.CharLimit = 2000
	ti.Width = 80

	return model{
		state:     stateInput,
		textInput: ti,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "q":
			if m.state != stateInput {
				m.quitting = true
				return m, tea.Quit
			}
		case "enter":
			if m.state == stateInput {
				m.url = strings.TrimSpace(m.textInput.Value())
				return m, tea.Quit
			}
			if m.state == stateSelectType && m.listReady {
				if i, ok := m.list.SelectedItem().(typeOption); ok {
					m.choiceType = i
					return m, tea.Quit
				}
			}
			if m.state == stateSelectRes && m.listReady {
				if i, ok := m.list.SelectedItem().(resolutionOption); ok {
					m.choiceRes = i
					return m, tea.Quit
				}
			}
			if m.state == stateSelectFPS && m.listReady {
				if i, ok := m.list.SelectedItem().(fpsOption); ok {
					m.choiceFPS = i
					return m, tea.Quit
				}
			}
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.listReady {
			h, v := docStyle.GetFrameSize()
			extraH := 0
			if m.notice != "" {
				extraH = lipgloss.Height(warnStyle.Render(m.notice))
			}
			m.list.SetSize(msg.Width-h, msg.Height-v-extraH)
		}
	}

	var cmd tea.Cmd
	if m.state == stateInput {
		m.textInput, cmd = m.textInput.Update(msg)
	} else if m.listReady {
		m.list, cmd = m.list.Update(msg)
	}
	return m, cmd
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	switch m.state {
	case stateInput:
		return fmt.Sprintf(
			"\n %s\n\n%s\n\n%s\n%s\n\n%s",
			titleStyle.Render("YouTube Downloader (H.264 Only)"),
			m.textInput.View(),
			"Enter a single URL, multiple URLs (comma-separated), or a path to a .txt file.",
			"Example: https://youtu.be/abc, https://youtu.be/xyz",
			"(Press Enter to begin selection, Ctrl+C to quit)",
		)
	case stateSelectType, stateSelectRes, stateSelectFPS:
		if !m.listReady {
			return "\n  Initializing selection list..."
		}
		
		if m.notice != "" {
			return lipgloss.JoinVertical(lipgloss.Left,
				"\n"+warnStyle.Render(m.notice),
				docStyle.Render(m.list.View()),
			)
		}
		return docStyle.Render(m.list.View())
	}
	return ""
}

func getVideoTitle(url string) string {
	cmd := exec.Command("bash", "-c", fmt.Sprintf("yt-dlp --get-title --cookies-from-browser chrome %s", url))
	out, err := cmd.Output()
	if err != nil {
		return "Unknown Title"
	}
	return strings.TrimSpace(string(out))
}

func fetchFormatsAndMatch(url string, targetHeight int, isAudioOnly bool) (string, []fpsOption, string, error) {
	title := getVideoTitle(url)
	cmd := exec.Command("bash", "-c", fmt.Sprintf("yt-dlp -F --cookies-from-browser chrome %s", url))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return title, nil, "", fmt.Errorf("failed to fetch formats: %v", err)
	}

	lines := strings.Split(string(out), "\n")
	var avcFormats []fpsOption
	var audioFormats []formatInfo

	for _, line := range lines {
		if strings.HasPrefix(line, "ID") || strings.HasPrefix(line, "[") || line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		id := parts[0]
		ext := parts[1]

		if strings.Contains(line, "audio only") {
			tbr := 0.0
			tbrRegex := regexp.MustCompile(`(\d+)k`)
			tbrMatch := tbrRegex.FindStringSubmatch(line)
			if len(tbrMatch) > 1 {
				tbr, _ = strconv.ParseFloat(tbrMatch[1], 64)
			}
			audioFormats = append(audioFormats, formatInfo{id: id, ext: ext, tbr: tbr})
			continue
		}

		if isAudioOnly {
			continue
		}

		// Filter for MP4 and AVC1
		if strings.Contains(line, "video only") && ext == "mp4" && strings.Contains(line, "avc1") {
			res := parts[2]
			resParts := strings.Split(res, "x")
			h := 0
			if len(resParts) > 1 {
				h, _ = strconv.Atoi(resParts[1])
			}

			if h != targetHeight {
				continue
			}

			fps := 0
			if len(parts) > 3 {
				if val, err := strconv.Atoi(parts[3]); err == nil {
					fps = val
				}
			}

			note := ""
			if idx := strings.LastIndex(line, "│"); idx != -1 {
				note = strings.TrimSpace(line[idx+1:])
			}

			avcFormats = append(avcFormats, fpsOption{
				id:    id,
				fps:   fps,
				label: fmt.Sprintf("%dfps", fps),
				note:  note,
			})
		}
	}

	var selectedAudioID string
	for _, f := range audioFormats {
		if f.id == "140" {
			selectedAudioID = "140"
			break
		}
	}
	if selectedAudioID == "" {
		sort.Slice(audioFormats, func(i, j int) bool { return audioFormats[i].tbr > audioFormats[j].tbr })
		for _, f := range audioFormats {
			if f.ext == "m4a" {
				selectedAudioID = f.id
				break
			}
		}
	}

	return title, avcFormats, selectedAudioID, nil
}

type formatInfo struct {
	id, ext string
	tbr     float64
}

func main() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\n\nOperation interrupted. Exiting...")
		os.Exit(0)
	}()

	// 1. URL Input
	mInput := initialModel()
	pInput := tea.NewProgram(mInput, tea.WithAltScreen())
	resInput, err := pInput.Run()
	if err != nil {
		log.Fatal(err)
	}
	finalInput := resInput.(model)
	if finalInput.quitting || finalInput.url == "" {
		return
	}

	input := finalInput.url
	var initialURLs []string
	if _, err := os.Stat(input); err == nil {
		file, _ := os.Open(input)
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			u := strings.TrimSpace(scanner.Text())
			if u != "" {
				initialURLs = append(initialURLs, u)
			}
		}
		file.Close()
	} else if strings.Contains(input, ",") {
		parts := strings.Split(input, ",")
		for _, p := range parts {
			u := strings.TrimSpace(p)
			if u != "" {
				initialURLs = append(initialURLs, u)
			}
		}
	} else {
		initialURLs = append(initialURLs, input)
	}

	activeURLs := initialURLs
	
	// Data for Selection Menus
	typeOptions := []list.Item{
		typeOption{label: "clip", isAudio: false},
		typeOption{label: "sound", isAudio: true},
	}
	resOptions := []list.Item{
		resolutionOption{label: "4320p (8K)", height: 4320},
		resolutionOption{label: "2160p (4K)", height: 2160},
		resolutionOption{label: "1440p (2K)", height: 1440},
		resolutionOption{label: "1080p", height: 1080},
		resolutionOption{label: "720p", height: 720},
		resolutionOption{label: "480p", height: 480},
		resolutionOption{label: "360p", height: 360},
		resolutionOption{label: "240p", height: 240},
	}

	firstPass := true

	for len(activeURLs) > 0 {
		var preQueue []downloadTask

		if firstPass {
			// --- GLOBAL SELECTION ---
			lType := list.New(typeOptions, list.NewDefaultDelegate(), 0, 0)
			lType.Title = "Select Default Download Type for all links"
			mType := model{state: stateSelectType, list: lType, listReady: true}
			pType := tea.NewProgram(mType, tea.WithAltScreen())
			resType, _ := pType.Run()
			finalType := resType.(model)
			if finalType.quitting {
				return
			}

			globalIsAudio := finalType.choiceType.isAudio
			globalHeight := 0

			if !globalIsAudio {
				lRes := list.New(resOptions, list.NewDefaultDelegate(), 0, 0)
				lRes.Title = "Select Default Resolution for all links"
				mRes := model{state: stateSelectRes, list: lRes, listReady: true}
				pRes := tea.NewProgram(mRes, tea.WithAltScreen())
				resRes, _ := pRes.Run()
				finalRes := resRes.(model)
				if finalRes.quitting {
					return
				}
				globalHeight = finalRes.choiceRes.height
			}

			for _, url := range activeURLs {
				preQueue = append(preQueue, downloadTask{
					url:          url,
					targetHeight: globalHeight,
					isAudioOnly:  globalIsAudio,
				})
			}
			firstPass = false
		} else {
			// --- INDIVIDUAL RETRY SELECTION ---
			for i, url := range activeURLs {
				notice := "⚠️  GLOBAL RESOLUTION NOT AVAILABLE FOR THIS CLIP"
				title := getVideoTitle(url)
				
				lType := list.New(typeOptions, list.NewDefaultDelegate(), 0, 0)
				lType.Title = fmt.Sprintf("[%d/%d] Select Alternative Type for: %s", i+1, len(activeURLs), title)
				mType := model{state: stateSelectType, list: lType, url: url, listReady: true, notice: notice}
				pType := tea.NewProgram(mType, tea.WithAltScreen())
				resType, _ := pType.Run()
				finalType := resType.(model)
				if finalType.quitting {
					return
				}

				task := downloadTask{url: url, isAudioOnly: finalType.choiceType.isAudio}

				if !task.isAudioOnly {
					lRes := list.New(resOptions, list.NewDefaultDelegate(), 0, 0)
					lRes.Title = fmt.Sprintf("[%d/%d] Select Alternative Resolution for: %s", i+1, len(activeURLs), title)
					mRes := model{state: stateSelectRes, list: lRes, url: url, listReady: true, notice: notice}
					pRes := tea.NewProgram(mRes, tea.WithAltScreen())
					resRes, _ := pRes.Run()
					finalRes := resRes.(model)
					if finalRes.quitting {
						return
					}
					task.targetHeight = finalRes.choiceRes.height
				}
				preQueue = append(preQueue, task)
			}
		}

		// Processing Phase
		var nextPassURLs []string
		fmt.Printf("\nProcessing %d tasks...\n", len(preQueue))
		for i, task := range preQueue {
			fmt.Printf("\n[%d/%d] Fetching formats for: %s\n", i+1, len(preQueue), task.url)
			title, avcFormats, audioID, err := fetchFormatsAndMatch(task.url, task.targetHeight, task.isAudioOnly)
			if err != nil {
				fmt.Printf("Error: %v. Retrying selection later.\n", err)
				nextPassURLs = append(nextPassURLs, task.url)
				continue
			}

			if task.isAudioOnly {
				if audioID != "" {
					fmt.Printf("Downloading Sound: %s\n", title)
					cmdStr := fmt.Sprintf("yt-dlp -f %s --cookies-from-browser chrome -o '%%(title)s.%%(ext)s' %s", audioID, task.url)
					cmd := exec.Command("bash", "-c", cmdStr)
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					cmd.Run()
				} else {
					fmt.Println("No audio format found. Retrying selection later.")
					nextPassURLs = append(nextPassURLs, task.url)
				}
				continue
			}

			// Video logic
			var selectedVideoID string
			if len(avcFormats) == 0 {
				fmt.Printf("No H.264 format found for %dp. Retrying selection later.\n", task.targetHeight)
				nextPassURLs = append(nextPassURLs, task.url)
				continue
			} else if len(avcFormats) == 1 {
				selectedVideoID = avcFormats[0].id
				fmt.Printf("Auto-selected: %s (%s)\n", avcFormats[0].label, title)
			} else {
				items := make([]list.Item, len(avcFormats))
				for j, f := range avcFormats {
					items[j] = f
				}
				lFPS := list.New(items, list.NewDefaultDelegate(), 0, 0)
				lFPS.Title = fmt.Sprintf("Multiple FPS found for %dp. Select for: %s", task.targetHeight, title)
				mFPS := model{state: stateSelectFPS, list: lFPS, listReady: true}
				pFPS := tea.NewProgram(mFPS, tea.WithAltScreen())
				resFPS, _ := pFPS.Run()
				finalFPS := resFPS.(model)
				if finalFPS.quitting {
					return
				}
				selectedVideoID = finalFPS.choiceFPS.id
			}

			if selectedVideoID != "" && audioID != "" {
				fmt.Printf("Downloading Clip: %s\n", title)
				cmdStr := fmt.Sprintf("yt-dlp -f %s+%s --cookies-from-browser chrome --merge-output-format mp4 --postprocessor-args \"ffmpeg:-c:v copy -c:a copy -map 0:v:0 -map 1:a:0 -pix_fmt yuv420p -shortest\" -o '%%(title)s.%%(ext)s' %s", selectedVideoID, audioID, task.url)
				cmd := exec.Command("bash", "-c", cmdStr)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				cmd.Run()
			}
		}

		activeURLs = nextPassURLs
		if len(activeURLs) > 0 {
			fmt.Printf("\n--- %d clips failed resolution check. Looping back to Individual Selection ---\n", len(activeURLs))
		}
	}

	fmt.Println("\n--- All Tasks Completed ---")
}
