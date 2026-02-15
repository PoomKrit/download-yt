# YouTube Downloader (download-yt)

A terminal-based YouTube downloader written in Go, featuring a rich interactive UI (TUI) and a focus on high-compatibility H.264 (AVC) video formats.

## What it is

`download-yt` is a CLI tool that simplifies downloading YouTube videos and audio. It provides an interactive interface to select download types (Video+Audio or Audio Only) and resolutions, ensuring that the resulting files are in the widely compatible MP4 (H.264/AVC) format.

## How it works

1. **Input Handling**: Accepts a single URL, a comma-separated list of URLs, or a path to a `.txt` file containing URLs.
2. **Interactive Selection**: Uses the Bubble Tea framework to provide a TUI for selecting:
   - **Download Type**: "clip" (Video + Sound) or "sound" (Audio only).
   - **Resolution**: From 240p up to 8K (4320p).
   - **FPS**: Allows choosing between different frame rates (e.g., 30fps vs 60fps) if multiple are available for the target resolution.
3. **Smart Filtering**: Specifically searches for `avc1` (H.264) video streams to ensure the downloaded MP4 files play natively on almost any device without needing special codecs.
4. **Backend Processing**: 
   - Uses `yt-dlp` to fetch metadata and download streams.
   - Uses `ffmpeg` to merge video and audio streams seamlessly.
   - Leverages Chrome browser cookies (`--cookies-from-browser chrome`) to handle restricted content and avoid bot detection.

## Prerequisites

Before installing, ensure you have the following tools installed and available in your system's PATH:

- **Go**: Version 1.25 or later (as specified in `go.mod`).
- **yt-dlp**: The primary download engine.
- **ffmpeg**: Required for merging video and audio streams.
- **Google Chrome**: The tool is configured to use cookies from Chrome to facilitate downloads.

## Steps to Install

1. **Clone the repository**:
   ```bash
   git clone <repository-url>
   cd download-yt
   ```

2. **Download dependencies**:
   ```bash
   go mod download
   ```

3. **Build the application**:
   ```bash
   go build -o download-yt
   ```
   *(Alternatively, you can run it directly using `go run main.go`)*

## Steps to Use

1. **Launch the tool**:
   ```bash
   ./download-yt
   ```

2. **Provide your links**:
   - Paste a **single URL**.
   - Paste **multiple URLs** separated by commas (e.g., `url1, url2`).
   - Provide a **path to a .txt file** containing one URL per line.

3. **Configure your download**:
   - **Select Type**: Choose between "clip" (Video) or "sound" (Audio).
   - **Select Resolution**: Pick your preferred quality (e.g., 1080p).
   - **Select FPS**: If multiple frame rates are available for your chosen resolution, select the one you prefer.

4. **Sit back and relax**: The tool will process each link, fetch the appropriate formats, and download them to your current directory. If a specific resolution isn't available for a clip, it will automatically prompt you to choose an alternative for that specific item.
