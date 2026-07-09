# **Autonomous CLI Architecture for Microsoft Teams Live Transcription and Audio Capture**

## **Executive Summary**

The demand for programmatic extraction of meeting transcripts and audio has accelerated significantly as organizations seek to integrate real-time communication data into Large Language Models (LLMs), automated compliance workflows, and custom analytics pipelines. While Microsoft provides official channels for transcript retrieval—most notably the Microsoft Graph API—these native solutions are encumbered by significant operational friction. The Graph API requires tenant-level administrator consent, mandates specific Microsoft 365 licensing tiers, and critically, only provides access to transcript data after a meeting has fully concluded1.  
To overcome these limitations and capture speech-to-text data and the raw audio stream in real-time, a synthetic participant architecture is required. This involves deploying a headless browser automation tool that programmatically joins a Microsoft Teams meeting, navigates the web application's Document Object Model (DOM), activates the live closed-captioning feature, intercepts the rendered text as it is streamed to the screen, and captures the meeting's internal audio1.  
This report provides an exhaustive architectural blueprint for constructing a standalone Command Line Interface (CLI) application in the Go programming language to achieve this objective. To strictly adhere to the operational constraints of generating a truly portable, standalone binary executable that requires no system-level package installations (such as FFmpeg), this design pivots away from traditional high-level frameworks like Playwright. Instead, it leverages chromedp, a pure Go implementation of the Chrome DevTools Protocol (CDP)4. The resulting architecture defines a highly resilient finite state machine capable of autonomous authentication, scheduled execution, DOM mutation observation, linguistic deduplication, continuous Markdown file generation, and pure-Go MP3 audio encoding.

## **The Paradigm of Headless Browser Automation**

The extraction of data from modern Single Page Applications (SPAs) like Microsoft Teams requires a full browser engine to execute JavaScript, hydrate the DOM, and maintain persistent WebSocket connections for media streaming6. Traditional HTTP-based scrapers are entirely ineffective against such platforms7.

### **Evaluating the Core Execution Engine**

The initial operational parameter suggested the use of Playwright via the Go programming language (playwright-go). While Playwright is a robust, cross-browser automation framework maintained by Microsoft, its architectural dependencies make it fundamentally incompatible with the requirement for a zero-dependency, standalone CLI tool8.  
Playwright operates as a Node.js abstraction layer that communicates with browser binaries6. When utilizing the playwright-go binding, the host machine is still required to download and manage specific, customized builds of Chromium, Firefox, or WebKit, which are stored in local cache directories (e.g., \~/.cache/ms-playwright/)9. The initialization process involves executing commands such as playwright.Install(), which fetches hundreds of megabytes of external dependencies9. Furthermore, Playwright heavily injects custom JavaScript payloads into the browser context to maintain cross-browser compatibility, resulting in heavier memory footprints and larger network payloads between the client and the browser engine14.  
To satisfy the strict requirement of an executable that can be seamlessly copied from machine to machine without requiring installation scripts or external package managers, the architecture must utilize chromedp9.

| Architectural Metric | playwright-go Architecture | chromedp Architecture | Viability for Standalone CLI |
| :---- | :---- | :---- | :---- |
| **Binary Portability** | Low. Functions as a wrapper; requires downloading extensive driver and browser binaries upon first execution9. | High. Written in pure Go; compiles to a single, static binary with no external library dependencies5. | chromedp strictly aligns with the zero-dependency, portable executable mandate. |
| **Browser Dependency** | Downloads specific, customized browser builds to a local user cache directory11. | Utilizes the host operating system's existing Chrome, Edge, or Brave installation automatically4. | chromedp prevents redundant browser installations and heavy initializations. |
| **Protocol Overhead** | Custom abstraction layer over CDP and WebDriver BiDi, utilizing extensive JavaScript injection14. | Direct implementation of the native Chrome DevTools Protocol (CDP) via JSON over WebSockets9. | chromedp provides lower-level, faster execution with reduced overhead14. |
| **Memory Footprint** | Higher due to context isolation mechanisms and extensive JavaScript payloads injected into the page14. | Lower. Executes native CDP commands directly against the browser's debug port4. | chromedp is highly optimized for long-running processes on resource-constrained machines4. |

By relying on chromedp, the Go application will search the host system for an existing installation of a Chromium-based browser (such as Google Chrome or Microsoft Edge) using the chromedp.DefaultExecAllocatorOptions18. It will then launch this existing binary in a headless state and drive it directly via the DevTools WebSocket interface, maintaining the purity of the standalone executable model.

## **CLI Orchestration and Daemon Scheduling**

A robust CLI tool requires a structured command routing system and a reliable mechanism for background scheduling. The application must operate seamlessly in both interactive environments and automated, unattended contexts.

### **Command Routing with Cobra**

The Go ecosystem's standard for complex command-line interfaces is the spf13/cobra package20. Cobra provides a rigid, hierarchical structure for defining commands, nested subcommands, and sophisticated flag parsing. The CLI architecture will be partitioned into three primary operational modes:

1. **Authentication Mode (auth):** An interactive command that launches the browser in a visible (headed) state, allowing the user to log into Microsoft Teams manually and cache the session credentials locally.  
2. **Immediate Execution Mode (join):** A command designed to accept a Microsoft Teams URL as an argument and instantaneously execute the browser automation sequence to join the meeting and record transcripts.  
3. **Daemon/Scheduler Mode (schedule):** A long-running background process that parses a configuration file and executes meeting-join routines based on predefined chronological triggers.

### **Task Scheduling with Cron Integration**

To fulfill the requirement of automatically joining meetings based on a configured schedule, the CLI will integrate the robfig/cron/v3 package20. This library provides a time-based job scheduler for Go, executing functions according to standard Unix cron syntax22.  
The implementation will utilize a structured configuration file (preferably YAML or JSON) stored in a standardized local directory (e.g., \~/.config/teams-cli/config.yaml). This file will define a list of target meetings, mapping the meeting URL to a specific cron expression23.  
When the user invokes the schedule command, the Go application will instantiate a new cron engine, parse the configuration file, and attach a discrete goroutine to each scheduled event. When the cron trigger fires, the scheduler will invoke the chromedp allocation logic, spawn the headless browser, and execute the meeting interaction sequence22. This architecture allows the tool to run indefinitely as a background service or a systemd daemon, autonomously capturing transcripts for a recurring roster of meetings.

## **Authentication and State Persistence**

Microsoft Teams enforces rigorous authentication protocols. While it is technically possible to program a bot to input a username and password during every execution, doing so programmatically on a recurring basis introduces severe operational risks. Repeated, automated login attempts frequently trigger Microsoft's bot detection heuristics, resulting in account lockouts, mandatory password resets, or the presentation of complex Multi-Factor Authentication (MFA) challenges that headless browsers cannot autonomously resolve3.  
To bypass the login sequence for unattended operations, the architecture must persist the browser's session state to the local file system26.

### **The User Data Directory Mechanism**

In the Chromium architecture, user state—including cookies, local storage, IndexedDB, and session tokens—is maintained within a specific directory profile. The Chrome DevTools Protocol allows developers to specify a custom directory for this state using the UserDataDir allocation option26.  
The implementation strategy will rely on a dedicated local storage path, such as \~/.local/share/teams-cli/profile/. The initial setup of the tool will mandate an interactive onboarding phase:

1. The user executes the command teams-cli auth.  
2. The Go application allocates a new chromedp context, explicitly disabling the headless flag (chromedp.Flag("headless", false)), and specifying the custom UserDataDir28.  
3. The visible browser navigates to https://teams.microsoft.com30.  
4. The user manually completes the Microsoft 365 authentication flow, interacting with any required MFA applications or SMS codes.  
5. Upon successful authentication, the user closes the browser. The session cookies and Azure Active Directory tokens are permanently written to the local profile directory26.

For all subsequent executions—whether triggered manually via the join command or automatically via the schedule daemon—the Go application will launch the browser in strict headless mode, injecting the exact same UserDataDir27. Because the session tokens remain valid on the disk, the browser will seamlessly bypass the Microsoft login screen and proceed directly to the meeting interface31.

## **Browser Instantiation and Stealth Mechanics**

Enterprise platforms like Microsoft Teams deploy sophisticated telemetry to detect automated access. If a headless browser is detected, the platform may block entry to the meeting, degrade the rendering of the DOM, or refuse to instantiate the WebRTC connections necessary for audio processing.

### **Overcoming Hardware Detection and Sandbox Restrictions**

When the Go application initializes the browser via chromedp.NewExecAllocator, it must inject a specific array of command-line arguments to stabilize the headless environment, grant audio capture permissions, and spoof the presence of physical hardware3.  
Microsoft Teams inherently expects access to a microphone and a camera. If it detects a headless environment lacking these peripherals, it will generate blocking permission prompts or halt the application initialization entirely8. Furthermore, headless browsers default to absolute silence, which actively prevents internal audio recording33.  
The following Chromium flags must be strictly applied during the allocation phase8:

| Chromium Command-Line Flag | Architectural Purpose |
| :---- | :---- |
| \--headless=new | Forces the utilization of the modernized Chrome headless architecture, which more accurately mimics a headed browser's rendering pipeline. |
| \--no-sandbox | Disables the Chrome security sandbox. This is critical for preventing startup crashes when the CLI is executed in restricted, containerized, or daemonized OS environments32. |
| \--disable-setuid-sandbox | Works in tandem with \--no-sandbox to ensure seamless execution across diverse Linux distributions without requiring root privileges35. |
| \--use-fake-ui-for-media-stream | Automatically suppresses and grants approval to any browser-level permission prompts requesting access to the microphone or camera8. |
| \--use-fake-device-for-media-stream | Injects a virtual media stream into the browser process. This provides Teams with a synthetic silent audio track and a blank video feed, satisfying its hardware dependency checks without requiring physical devices8. |
| chromedp.Flag("mute-audio", false) | By default, headless Chrome aggressively mutes all tab audio. This flag forcefully un-mutes the browser so the internal audio stream can be intercepted33. |
| \--auto-select-desktop-capture-source="Microsoft Teams" | Automatically suppresses and approves the getDisplayMedia screen-sharing prompt, allowing the bot to capture the tab's audio stream without human intervention33. |
| \--disable-blink-features=AutomationControlled | A critical stealth flag that removes the navigator.webdriver \= true property from the JavaScript runtime, heavily masking the presence of the CDP automation engine36. |

### **User-Agent Standardization**

The DOM structure of Microsoft Teams is highly dynamic, frequently altering its CSS classes and element hierarchies based on the rendering browser's User-Agent string35. If the tool executes on a macOS machine, it may receive a different DOM structure than if executed on a Linux server.  
To guarantee that the CSS selectors utilized by the Go scraping logic remain perfectly consistent regardless of the host operating system, the application must force a standardized User-Agent string35. By utilizing chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36"), the Teams server will reliably deliver the standard Windows Chromium DOM, neutralizing a significant vector for automation failure29.

## **The Finite State Machine: Navigating Microsoft Teams**

Joining a Microsoft Teams meeting via an external link is a multi-stage process involving complex redirects, Single Page Application (SPA) state changes, and interstitial prompts. The Go application must implement a rigorous Finite State Machine (FSM) utilizing chromedp.Tasks to traverse these phases reliably.

### **Phase 1: URL Mutation and Interstitial Evasion**

When a user or a bot navigates to a standard Microsoft Teams meeting link (e.g., https://teams.microsoft.com/l/meetup-join/...), the server does not immediately route to the web client. Instead, it presents an interstitial landing page designed to aggressively prompt the user to "Open the Teams App" or "Download the Desktop App"32.  
Headless browsers cannot interact with deep-link application protocols (such as msteams://). Therefore, attempting to click the "Continue on this browser" button via DOM selectors is highly unstable, as the application prompt often intercepts the execution context37.  
The definitive architectural solution is to programmatically mutate the meeting URL before initiating the browser navigation. By appending a specific set of query parameters to the target URL, the CLI tool instructs the Microsoft Teams routing engine to bypass the desktop application launcher entirely and render the web-based SPA directly8.  
The Go URL parsing logic must append the following parameters35:

* msLaunch=false  
* type=meetup-join  
* directDl=true  
* enableMobilePage=true  
* suppressPrompt=true

### **Phase 2: Orchestrating the Pre-Join Interface**

Upon successful navigation via the mutated URL, the browser will render the "Pre-Join" interface. This screen requires the user to configure audio/video peripherals and input a display name if they are an unauthenticated guest38.  
The FSM must execute the following sequential logic:

| Action Sequence | DOM Target / Selector | CDP Execution |
| :---- | :---- | :---- |
| **DOM Hydration Wait** | The primary pre-join container module. | chromedp.WaitVisible ensures the React components have fully rendered before interaction41. |
| **Hardware Toggles** | The microphone and camera toggle switches43. | The tool must evaluate the state of these toggles (often via aria-checked attributes). If active, the tool executes chromedp.Click to ensure the bot enters the meeting muted, preventing unintended audio transmission13. |
| **Identity Injection** | input\[placeholder="Enter name"\] or similar semantic selector39. | If the session is unrecognized, the tool executes chromedp.SendKeys to inject a configured bot name (e.g., "Transcription Agent")39. |
| **Meeting Entry** | The "Join now" primary action button47. | chromedp.Click triggers the final network request to instantiate the WebRTC connection and enter the meeting39. |

### **Phase 3: Lobby Resolution and State Polling**

Clicking "Join now" does not guarantee immediate access to the meeting audio. Depending on the tenant's security configuration, the bot will transition to one of two states:

1. **Immediate Admission:** The bot directly accesses the meeting interface.  
2. **The Virtual Lobby:** The bot is sequestered in a waiting room, greeted by a notification such as "Someone will let you in shortly"32.

Because the Go application cannot predict which state will occur, it must deploy a race condition observer. The chromedp context will utilize a polling loop, checking the DOM simultaneously for the presence of the main meeting control toolbar (indicating success) and the lobby text string (indicating a holding pattern)35. If the lobby state is identified, the application will emit a log to the CLI and wait indefinitely—or until a configured timeout limit—for a human organizer to grant admission from the inside35.

## **Live Caption Extraction: The MutationObserver Paradigm**

A central capability of this architecture is the real-time extraction of spoken text. Microsoft Teams does not expose a local WebSocket or API endpoint for this data during the call; instead, it renders live captions directly into the DOM, updating the text dynamically as its internal Azure Cognitive Services Speech-to-Text (STT) engine processes the audio stream1.

### **Programmatic Activation of Live Captions**

Once the FSM confirms admission to the active meeting, the bot must autonomously enable the captioning engine. Because the Microsoft Teams interface relies heavily on semantic, localized aria-labels and dynamically generated class names, targeting the UI elements requires precise selector strategies.  
The Go tool will execute a sequence of chromedp.Click operations:

1. Locate and click the "More actions" button in the primary meeting control toolbar (typically represented by an ellipsis ... icon)50.  
2. Navigate the subsequent pop-out menu and click the "Language and speech" option50.  
3. Click the final "Turn on live captions" button15.

### **Injecting the MutationObserver**

Attempting to scrape the captions by polling the DOM from the Go application at a fixed interval (e.g., executing chromedp.Text every 500ms) is fundamentally flawed. It generates massive CPU overhead, floods the CDP WebSocket connection, and frequently misses rapid or fleeting text updates53.  
The optimal architectural pattern is to inject a native JavaScript MutationObserver directly into the browser's V8 execution context32. This shifts the computational burden of monitoring the DOM entirely to the browser engine, which will asynchronously push data back to the Go application only when a change occurs.  
The Microsoft Teams caption container is wrapped within an element identifiable by the specific attribute data-tid="closed-caption-renderer-wrapper"35. Within this parent wrapper, individual caption blocks are dynamically generated. Each block generally utilizes the class .fui-ChatMessageCompact and contains child elements indicating the speaker's name (span\[data-tid="author"\]) and the spoken text (span\[data-tid="closed-caption-text"\])35.  
The Go application will deploy chromedp.Evaluate to execute a JavaScript payload46. This script will instantiate the MutationObserver, targeting the closed-caption-renderer-wrapper and configuring it to monitor for childList (new caption blocks being added) and characterData (existing blocks being updated with new words) mutations35.

### **Linguistic Deduplication: The Rolling Text Heuristic**

A critical technical complication in scraping live captions from an STT engine is the "rolling text" phenomenon. As a participant speaks, the STT engine continuously refines its confidence in the transcription. Consequently, Teams does not wait for a full sentence to finish before rendering it; it updates the exact same DOM element multiple times with expanding partial fragments35.  
For example, a single spoken phrase will trigger a rapid sequence of DOM mutations:

1. "The architecture"  
2. "The architecture requires"  
3. "The architecture requires a standalone"  
4. "The architecture requires a standalone binary."

If the MutationObserver blindly captures and transmits every single mutation, the resulting transcript file will be incomprehensibly bloated with overlapping, redundant fragments35.  
To resolve this, a localized linguistic heuristic algorithm must be embedded within the injected JavaScript payload. This algorithm relies on the detection of terminal punctuation. The Microsoft Teams STT engine systematically applies terminal punctuation (periods, question marks, exclamation points) when it determines a phonetic pause or the definitive end of a semantic clause35.  
The deduplication logic operates as follows:

1. **Ingestion:** The observer reads the incoming text mutation from the closed-caption-text span35.  
2. **Sanitization:** The script strips intermediate punctuation and compares the raw alphanumeric string against a previously cached variable to aggressively filter out exact duplicate mutations35.  
3. **Terminal Evaluation:** The sanitized string is evaluated against a regular expression targeting terminal punctuation markers (e.g., /\[.,\!?\]/g)35.  
4. **Finalization:** If terminal punctuation is detected, the fragment is classified as a finalized utterance. The script aggregates the author's name, the current timestamp, and the text into a JSON object, preparing it for exfiltration35.

## **Real-Time Audio Capture and MP3 Encoding**

Beyond text, preserving the acoustic context of the meeting requires capturing the browser tab's audio output. Because this CLI tool must remain a strictly standalone binary with no external CGO or FFmpeg dependencies55, the architecture leverages in-browser recording paired with pure Go encoding.

### **Stream Interception via MediaDevices API**

Once the bot is inside the meeting, the Go application uses chromedp.Evaluate to inject a second JavaScript payload targeting the browser's WebRTC API. Specifically, it invokes navigator.mediaDevices.getDisplayMedia({ audio: true, video: true })33. Chromium mandates that the video: true parameter be present for this API to fire on desktop streams; however, the script instantly extracts the audio track (stream.getAudioTracks()\[0\]) and discards the video stream to save computational overhead56.  
Because the \--auto-select-desktop-capture-source flag was set during the initial browser allocation, Chrome skips the standard screen-sharing UI prompt and automatically grants the script access to the tab's internal audio33.

### **Chunking the Audio Stream**

The isolated audio stream is passed into a native MediaRecorder instance configured with the audio/webm MIME type58. The recorder is set to emit chunks of audio data at regular intervals via its ondataavailable event handler59. As each chunk is generated, the JavaScript payload converts the Blob into a Base64-encoded string, readying it for the WebSockets bridge60.

### **Pure Go MP3 Conversion**

To format the audio into an MP3 without requiring users to install external software, the Go application utilizes a pure Go encoding library compiled directly into the binary. Packages like github.com/braheezy/shine-mp3 (a Go port of the Shine fixed-point MP3 encoder) or github.com/formeo/go-audio-converter provide statically compiled MP3 encoding with zero CGO dependencies55.

## **The JavaScript-to-Go Data Exfiltration Bridge**

With the MutationObserver finalizing text fragments and the MediaRecorder emitting Base64 audio chunks within the browser, the architecture requires an asynchronous, zero-latency bridge to transmit this data back to the compiled Go binary.  
Relying on HTTP callbacks or secondary local WebSocket servers introduces unnecessary network layers and security complications. Instead, the architecture leverages the Chrome DevTools Protocol's native runtime binding capabilities60.  
This mechanism allows the Go application to expose native functions directly into the browser's global window object.

1. **Go Protocol Setup:** Before injecting the JavaScript payloads, the Go code executes cdpruntime.AddBinding("sendCaptionToGo").Do(ctx) and cdpruntime.AddBinding("sendAudioToGo").Do(ctx)60. This instructs the V8 engine to listen for these specific function calls.  
2. **Event Listener Establishment:** The Go application establishes asynchronous listeners using chromedp.ListenTarget. These listeners filter the firehose of CDP traffic for \*cdpruntime.EventBindingCalled events65.  
3. **JavaScript Execution:** When the MutationObserver finalizes a caption fragment, it executes window.sendCaptionToGo(JSON.stringify(captionData)). Likewise, when the MediaRecorder generates a new chunk, it executes window.sendAudioToGo(base64AudioChunk)62.  
4. **Data Reception:** The Go listeners catch the EventBindingCalled events. They extract the payloads, decode them, and route them into specific internal Go channels (e.g., captionChan and audioChan) for file processing13.

## **Transcript Formatting and File I/O Management**

Dedicated worker goroutines sit at the end of the Go channels, assuming responsibility for formatting the data and writing it to the local file system.

### **Markdown Template Structuring**

The operational parameters require the tool to automatically save transcripts as text in Markdown format, utilizing a specific template structure. To achieve this cleanly, the application will leverage the Go standard library's text/template package.  
When a meeting is successfully joined, the application will create a new .md file (for text) and an .mp3 file (for audio) in the pre-configured target directory. The Markdown file will be dynamically named using a combination of the sanitized meeting ID and a localized timestamp. The text worker goroutine will immediately write a YAML frontmatter header to the file, defining critical metadata such as the meeting date, start time, and execution parameters.  
The incoming caption data will be decoded into a strict Go data structure:

Go  
type CaptionEvent struct {  
    Timestamp string  
    Speaker   string  
    Text      string  
}

As the CaptionEvent struct is populated, the worker will append the formatted string directly to the file stream.

| Metadata Field | Contextual Description | Markdown Output Mapping |
| :---- | :---- | :---- |
| **Meeting Date** | ISO 8601 formatted date of the meeting execution. | YAML Frontmatter block. |
| **Start Time** | Local system time the bot entered the meeting. | YAML Frontmatter block. |
| **Speaker** | The participant name extracted from data-tid="author". | \*\*\[Speaker Name\] (HH:MM:SS):\*\* |
| **Utterance** | The finalized, deduplicated text fragment. | Plain text block immediately following the speaker designation. |

### **Buffered I/O and Memory Safety**

It is critical that the architecture avoids holding the entire transcript string or audio stream in system memory. In enterprise scenarios, meetings can last for several hours, generating massive volumes of data. Aggregating this data in memory before writing it to disk creates a severe risk of memory bloat and subsequent Out-Of-Memory (OOM) panics10.  
By employing a buffered I/O mechanism (such as Go's bufio.Writer), both the text formatter and the pure Go MP3 encoder will continuously stream their processed bytes to the disk. The buffers will automatically flush their contents to the storage drive when they reach capacity or at regular chronological intervals, ensuring a near-zero memory footprint regardless of the meeting's duration10.

## **Process Lifecycle and Zombie Mitigation**

A notoriously frequent failure mode in headless browser automation—particularly when deployed as a background daemon or cron job—is the accumulation of orphaned or "zombie" Chromium processes. If the Go CLI tool encounters a fatal timeout, an unhandled exception, or receives an external termination signal (such as a user pressing Ctrl+C), the spawned Chromium child processes may remain active in the OS background10. Over time, these zombie processes will perpetually consume system RAM and CPU, eventually crashing the host machine10.  
To ensure the CLI remains an idempotent, clean, and professional utility, strict context lifecycle management is absolutely mandatory.  
The chromedp.ExecAllocator creates a temporary profile directory and spawns an OS-level binary process. The Go application must wrap the entire execution sequence within a top-level execution context featuring a robust deferment chain. By defining ctx, cancel := context.WithCancel(context.Background()) and immediately declaring defer cancel(), the Go runtime guarantees that the signal to terminate the browser process will be sent regardless of how the function exits.  
Furthermore, the CLI architecture must proactively intercept operating system signals. By utilizing the Go os/signal package to listen for interrupts (SIGTERM, SIGINT), the tool can execute a graceful shutdown sequence. Upon receiving an interrupt, the application will catch the signal, force the bufio.Writer and MP3 encoders to flush any remaining bytes to the disk, trigger the global context.Cancel() function to slaughter the headless browser process, and exit cleanly10.

## **Implementation Blueprint for Autonomous Agent (Handoff)**

This section provides the exhaustive, sequential architectural instructions required by the autonomous development agent, "Harness", to construct the standalone CLI application based on the established research parameters.

### **Phase 1: Scaffold the CLI Architecture and Scheduler**

1. **Module Initialization:** Initialize a new Go module. Ensure all external dependencies (including pure Go MP3 encoders like shine-mp3) are managed to allow for single-binary static compilation61.  
2. **CLI Framework (spf13/cobra):** Integrate the Cobra library to define the application's command hierarchy21. Construct the root command alongside three specific subcommands: auth (for interactive login), join (for immediate URL execution), and schedule (for daemonized execution).  
3. **Cron Integration (robfig/cron/v3):** Integrate the cron scheduler. Implement a configuration parser that reads a local file (e.g., config.yaml) to load a dictionary of meeting URLs mapped to their respective cron expressions20. Attach the execution payload to the cron triggers.  
4. **Configuration Management:** Implement localized configuration management allowing the user to dictate the default output directory for the generated Markdown and MP3 files.

### **Phase 2: Browser Orchestration and State Persistence**

1. **CDP Integration:** Import the github.com/chromedp/chromedp package4.  
2. **Allocator Construction:** Build the chromedp.ExecAllocatorOptions slice. Inject the strict array of stealth and media-spoofing flags (--no-sandbox, \--use-fake-ui-for-media-stream, \--use-fake-device-for-media-stream, \--disable-blink-features=AutomationControlled, \--auto-select-desktop-capture-source)8. Explicitly pass chromedp.Flag("mute-audio", false) to permit internal audio routing33. Set a standardized Windows Chromium User-Agent35.  
3. **Profile Persistence:** Implement the UserDataDir flag to point to a local directory (e.g., \~/.config/teams-bot/profile). This guarantees that session state, cookies, and tokens are saved to disk19.  
4. **Authentication Routine:** For the auth command, dynamically remove the \--headless flag. Launch the browser to https://teams.microsoft.com visibly, allowing the user to bypass MFA manually. Once the process is closed, the profile is permanently primed for headless execution3.

### **Phase 3: The Finite State Machine (Teams Interaction)**

1. **URL Mutation:** Implement a regex-based URL parser that intercepts standard Teams links and forcefully appends the bypass query parameters: ?msLaunch=false\&type=meetup-join\&directDl=true\&enableMobilePage=true\&suppressPrompt=true35.  
2. **Pre-Join Orchestration:** Construct the chromedp.Tasks sequence:  
   * Navigate to the mutated URL.  
   * WaitVisible for the core pre-join DOM elements68.  
   * Evaluate the aria-checked state of the microphone and camera toggles; execute chromedp.Click if they are active to enforce mute status13.  
   * If the display name input exists, inject the bot's configured name via chromedp.SendKeys47.  
   * Trigger chromedp.Click on the "Join now" selector47.  
3. **Lobby Resolution:** Implement a concurrent polling mechanism or WaitVisible array to detect either the primary meeting toolbar (signaling successful entry) or the text string "Someone will let you in shortly" (signaling lobby sequestration)35. Log the state transitions.

### **Phase 4: MutationObserver, Audio Capture, and Exfiltration**

1. **Caption Activation:** Once inside the meeting, execute sequential clicks targeting the aria-label or data-IDs for "More actions" (...), "Language and speech", and finally "Turn on live captions"52.  
2. **Bridge Establishment:** Execute cdpruntime.AddBinding("sendCaptionToGo") and cdpruntime.AddBinding("sendAudioToGo") to prime the V8 engine60. Establish chromedp.ListenTarget functions to filter for \*cdpruntime.EventBindingCalled events, routing the decoded JSON and Base64 payloads into separate Go channels13.  
3. **JavaScript Injection (Text):** Use chromedp.Evaluate to inject the observer payload. The script must:  
   * Instantiate a MutationObserver focused on the div\[data-tid="closed-caption-renderer-wrapper"\]35.  
   * Apply the terminal punctuation heuristic (/\[.,\!?\]/g) and raw string caching to deduplicate the rolling text35.  
   * Execute window.sendCaptionToGo(JSON.stringify(data)) when a fragment is finalized13.  
4. **JavaScript Injection (Audio):** Use chromedp.Evaluate to inject the recording payload. The script must:  
   * Call navigator.mediaDevices.getDisplayMedia({ audio: true, video: true }) and isolate the audio track56.  
   * Pass the track into a MediaRecorder (audio/webm)58.  
   * In the ondataavailable handler, convert the Blob to Base64 and execute window.sendAudioToGo(base64Audio)59.

### **Phase 5: Markdown/MP3 Generation and Teardown**

1. **File I/O Initialization:** Upon meeting entry, instantiate a bufio.Writer pointing to a newly created Markdown file, writing the template-driven YAML frontmatter. Simultaneously, initialize the pure Go MP3 encoder pointing to an .mp3 file55.  
2. **Stream Processing (Text):** Start a dedicated worker goroutine that listens to the text channel. Map CaptionEvent data to the structural template (Speaker Name (HH:MM:SS): Text) and append it to the text buffer.  
3. **Stream Processing (Audio):** Start a dedicated worker goroutine that listens to the audio channel, decodes the Base64 WebM/PCM chunks, and writes them through the MP3 encoder61.  
4. **Lifecycle Management:** Implement the os/signal intercepter. Ensure that the global context cancellation function (cancel()) is strictly deferred at the top of the execution stack to guarantee the slaughter of zombie Chromium processes upon termination, ensuring all I/O buffers are flushed safely10.

#### **Works cited**

1. When to build a Microsoft Teams bot with Puppeteer \- Recall.ai, [https://www.recall.ai/blog/puppeteer-microsoft-teams-bot](https://www.recall.ai/blog/puppeteer-microsoft-teams-bot)  
2. Microsoft Teams Transcription API · Open Source \- Vexa, [https://vexa.ai/product/microsoft-teams-transcription-api](https://vexa.ai/product/microsoft-teams-transcription-api)  
3. How to Build a Microsoft Teams Meeting Bot? \- MeetStream.ai, [https://meetstream.ai/blog/build-microsoft-teams-meeting-bot/](https://meetstream.ai/blog/build-microsoft-teams-meeting-bot/)  
4. Golang Headless Browser: Best Tools for Automation \- Latenode Blog, [https://latenode.com/blog/golang-headless-browser](https://latenode.com/blog/golang-headless-browser)  
5. ChromeDP Tutorial: Master Browser Automation in Go with Real-World Examples and Best Practices \- Rebrowser, [https://rebrowser.net/blog/chromedp-tutorial-master-browser-automation-in-go-with-real-world-examples-and-best-practices](https://rebrowser.net/blog/chromedp-tutorial-master-browser-automation-in-go-with-real-world-examples-and-best-practices)  
6. CDP vs Playwright vs Puppeteer: Is This the Wrong Question? \- Blog \- Lightpanda, [https://lightpanda.io/blog/posts/cdp-vs-playwright-vs-puppeteer-is-this-the-wrong-question](https://lightpanda.io/blog/posts/cdp-vs-playwright-vs-puppeteer-is-this-the-wrong-question)  
7. Best open-source web crawlers in 2026 \- Firecrawl, [https://www.firecrawl.dev/blog/best-open-source-web-crawler](https://www.firecrawl.dev/blog/best-open-source-web-crawler)  
8. Playwright Testing Hub: Blogs, Guides, Videos & Resources \- TestDino, [https://testdino.com/blog/playwright-testing-hub](https://testdino.com/blog/playwright-testing-hub)  
9. Playwright for Go \- BrowserStack, [https://www.browserstack.com/guide/playwright-go](https://www.browserstack.com/guide/playwright-go)  
10. Playwright for Go a browser automation library to control Chromium, Firefox and WebKit with a single API. \- GitHub, [https://github.com/playwright-community/playwright-go](https://github.com/playwright-community/playwright-go)  
11. The best browser automation framework, in every language | Browserbase, [https://www.browserbase.com/blog/browser-automation-all-languages-with-stagehand](https://www.browserbase.com/blog/browser-automation-all-languages-with-stagehand)  
12. atmos auth, [https://atmos.tools/cli/commands/auth/usage](https://atmos.tools/cli/commands/auth/usage)  
13. playwright package \- github.com/playwright-community/playwright-go \- Go Packages, [https://pkg.go.dev/github.com/playwright-community/playwright-go](https://pkg.go.dev/github.com/playwright-community/playwright-go)  
14. Playwright in Golang for Web Scraping \[Tutorial 2026\] \- ZenRows, [https://www.zenrows.com/blog/playwright-golang](https://www.zenrows.com/blog/playwright-golang)  
15. playwright package \- github.com/mxschmitt/playwright-go \- Go Packages, [https://pkg.go.dev/github.com/mxschmitt/playwright-go](https://pkg.go.dev/github.com/mxschmitt/playwright-go)  
16. Quickstart | Browserless Documentation, [https://docs.browserless.io/baas/quick-start](https://docs.browserless.io/baas/quick-start)  
17. dhamaniasad/HeadlessBrowsers: A list of (almost) all headless web browsers in existence \- GitHub, [https://github.com/dhamaniasad/HeadlessBrowsers](https://github.com/dhamaniasad/HeadlessBrowsers)  
18. chromedp/chromedp\_test.go at main \- GitHub, [https://github.com/chromedp/chromedp/blob/main/chromedp\_test.go](https://github.com/chromedp/chromedp/blob/main/chromedp_test.go)  
19. Chromedp Golang scraper not executing correctly \- Stack Overflow, [https://stackoverflow.com/questions/74482280/chromedp-golang-scraper-not-executing-correctly](https://stackoverflow.com/questions/74482280/chromedp-golang-scraper-not-executing-correctly)  
20. github.com/raiworks/rapidgo/v2 v2.12.1-0.20260704054444-d802bc102341 on Go \- Libraries.io, [https://libraries.io/go/github.com%2Fraiworks%2Frapidgo%2Fv2](https://libraries.io/go/github.com%2Fraiworks%2Frapidgo%2Fv2)  
21. How can I include Cobra in my actual project structure ? \#910 \- GitHub, [https://github.com/spf13/cobra/issues/910](https://github.com/spf13/cobra/issues/910)  
22. Scheduling Tasks with Cron | CodeSignal Learn, [https://codesignal.com/learn/courses/system-automation-with-shell-scripts/lessons/scheduling-tasks-with-cron](https://codesignal.com/learn/courses/system-automation-with-shell-scripts/lessons/scheduling-tasks-with-cron)  
23. Cron Jobs: The Complete Guide for 2026 \- Cronitor, [https://cronitor.io/guides/cron-jobs](https://cronitor.io/guides/cron-jobs)  
24. Scheduling tasks with cron and crontab in Linux \- Stackscale, [https://www.stackscale.com/blog/cron-crontab-linux/](https://www.stackscale.com/blog/cron-crontab-linux/)  
25. How can I schedule tasks to be done using the CLI (tw\_cli)? \- Broadcom Inc., [https://www.broadcom.com/support/knowledgebase/1211161452435/how-can-i-schedule-tasks-to-be-done-using-the-cli-twcli](https://www.broadcom.com/support/knowledgebase/1211161452435/how-can-i-schedule-tasks-to-be-done-using-the-cli-twcli)  
26. Persisting State | Browserless Documentation, [https://docs.browserless.io/baas/session-management/persisting-state](https://docs.browserless.io/baas/session-management/persisting-state)  
27. google chrome \- start browser with specicfic user profile using chromedp \- Stack Overflow, [https://stackoverflow.com/questions/45871304/start-browser-with-specicfic-user-profile-using-chromedp](https://stackoverflow.com/questions/45871304/start-browser-with-specicfic-user-profile-using-chromedp)  
28. Load up existing Chrome user profile & data with chromedp \- Stack Overflow, [https://stackoverflow.com/questions/59287313/load-up-existing-chrome-user-profile-data-with-chromedp](https://stackoverflow.com/questions/59287313/load-up-existing-chrome-user-profile-data-with-chromedp)  
29. Chromedp with Golang has deadline exceeded if element doesnt exist. Is there a way to extend the context timeout after a deadline? \- Stack Overflow, [https://stackoverflow.com/questions/70038292/chromedp-with-golang-has-deadline-exceeded-if-element-doesnt-exist-is-there-a-w](https://stackoverflow.com/questions/70038292/chromedp-with-golang-has-deadline-exceeded-if-element-doesnt-exist-is-there-a-w)  
30. How to log in to Microsoft Teams, [https://support.microsoft.com/en-us/teams/meetings/how-to-log-in-to-microsoft-teams](https://support.microsoft.com/en-us/teams/meetings/how-to-log-in-to-microsoft-teams)  
31. Exporting Live Captions from Microsoft Teams | Denis Molodtsov Blog, [https://spdenis.com/exporting-live-captions-from-microsoft-teams/](https://spdenis.com/exporting-live-captions-from-microsoft-teams/)  
32. How to build a Microsoft Teams Bot \- Recall.ai, [https://www.recall.ai/blog/how-to-build-a-microsoft-teams-bot](https://www.recall.ai/blog/how-to-build-a-microsoft-teams-bot)  
33. navigator.mediaDevices.getDisplayMedia({audio: true, video: true}) inside docker on gcloud, [https://stackoverflow.com/questions/74613616/navigator-mediadevices-getdisplaymediaaudio-true-video-true-inside-docker](https://stackoverflow.com/questions/74613616/navigator-mediadevices-getdisplaymediaaudio-true-video-true-inside-docker)  
34. No Sound · Issue \#1340 · chromedp/chromedp \- GitHub, [https://github.com/chromedp/chromedp/issues/1340](https://github.com/chromedp/chromedp/issues/1340)  
35. Releases · Zerg00s/Live-Captions-Saver \- GitHub, [https://github.com/Zerg00s/Live-Captions-Saver/releases](https://github.com/Zerg00s/Live-Captions-Saver/releases)  
36. ChromeDP Complete Guide: Advanced Web Automation & Scraping with Go \- Coronium.io, [https://www.coronium.io/blog/chromedp-complete-automation-guide](https://www.coronium.io/blog/chromedp-complete-automation-guide)  
37. How to join Microsoft Teams meetings without the app \- Supernormal, [https://www.supernormal.com/blog/joining-microsoft-teams-meetings-without-the-app](https://www.supernormal.com/blog/joining-microsoft-teams-meetings-without-the-app)  
38. 5 Easy Ways to Join a Microsoft Teams Meeting in 2026 \- ScreenApp, [https://screenapp.io/blog/join-microsoft-teams-meeting-guide](https://screenapp.io/blog/join-microsoft-teams-meeting-guide)  
39. Microsoft Teams \- Joining a Teams Meeting on the Web \- UW-Green Bay, [https://servicedesk.uwgb.edu/TDClient/33/Portal/KB/Article/4153/Microsoft-Teams-Joining-a-Teams-Meeting-on-the-Web](https://servicedesk.uwgb.edu/TDClient/33/Portal/KB/Article/4153/Microsoft-Teams-Joining-a-Teams-Meeting-on-the-Web)  
40. Join a meeting in Microsoft Teams, [https://support.microsoft.com/en-us/teams/meetings/join-a-meeting-in-microsoft-teams](https://support.microsoft.com/en-us/teams/meetings/join-a-meeting-in-microsoft-teams)  
41. How to build a visual uptime monitor with Go and headless Chrome \- GrabDiff, [https://grabdiff.com/blog/build-screenshot-monitor-go](https://grabdiff.com/blog/build-screenshot-monitor-go)  
42. google chrome \- Chromedp: handle alert \- Stack Overflow, [https://stackoverflow.com/questions/58085607/chromedp-handle-alert](https://stackoverflow.com/questions/58085607/chromedp-handle-alert)  
43. Use video in Microsoft Teams, [https://support.microsoft.com/en-us/teams/meetings/use-video-in-microsoft-teams](https://support.microsoft.com/en-us/teams/meetings/use-video-in-microsoft-teams)  
44. Manage audio settings in Microsoft Teams meetings, [https://support.microsoft.com/en-us/teams/meetings/manage-audio-settings-in-microsoft-teams-meetings](https://support.microsoft.com/en-us/teams/meetings/manage-audio-settings-in-microsoft-teams-meetings)  
45. Missing "Join" button in Microsoft Teams calendar for meetings from external Users | Microsoft Community Hub, [https://techcommunity.microsoft.com/discussions/microsoftteams/missing-join-button-in-microsoft-teams-calendar-for-meetings-from-external-users/897254/replies/2763689](https://techcommunity.microsoft.com/discussions/microsoftteams/missing-join-button-in-microsoft-teams-calendar-for-meetings-from-external-users/897254/replies/2763689)  
46. chromedp package \- github.com/chromedp/chromedp \- Go Packages, [https://pkg.go.dev/github.com/chromedp/chromedp](https://pkg.go.dev/github.com/chromedp/chromedp)  
47. Bypass "Join Now" confirmation | Microsoft Community Hub, [https://techcommunity.microsoft.com/discussions/microsoftteams/bypass-join-now-confirmation/1671703](https://techcommunity.microsoft.com/discussions/microsoftteams/bypass-join-now-confirmation/1671703)  
48. Using the lobby in Microsoft Teams meetings, [https://support.microsoft.com/en-us/teams/meetings/using-the-lobby-in-microsoft-teams-meetings](https://support.microsoft.com/en-us/teams/meetings/using-the-lobby-in-microsoft-teams-meetings)  
49. Teams Premium User Guide \- Microsoft 365 Adoption, [https://adoption.microsoft.com/files/microsoft-teams/Microsoft-Teams-Premium-user-guide.pdf](https://adoption.microsoft.com/files/microsoft-teams/Microsoft-Teams-Premium-user-guide.pdf)  
50. Use live captions in Microsoft Teams meetings, [https://support.microsoft.com/en-us/teams/meetings/use-live-captions-in-microsoft-teams-meetings](https://support.microsoft.com/en-us/teams/meetings/use-live-captions-in-microsoft-teams-meetings)  
51. Real-Time Translation Tools for Microsoft Teams Meetings (with Meeting Notes) | Blog, [https://super-intern.com/en/blog/2026-teams-realtime-translation-tools](https://super-intern.com/en/blog/2026-teams-realtime-translation-tools)  
52. playwright package \- github.com/hmajid2301/playwright-go \- Go Packages, [https://pkg.go.dev/github.com/hmajid2301/playwright-go](https://pkg.go.dev/github.com/hmajid2301/playwright-go)  
53. Lessons from managing hundreds of headless Chrome instances in Go : r/golang \- Reddit, [https://www.reddit.com/r/golang/comments/1rlala0/lessons\_from\_managing\_hundreds\_of\_headless\_chrome/](https://www.reddit.com/r/golang/comments/1rlala0/lessons_from_managing_hundreds_of_headless_chrome/)  
54. Web application settings \- Pendo Support, [https://support.pendo.io/hc/en-us/articles/22367036606619-Web-application-settings](https://support.pendo.io/hc/en-us/articles/22367036606619-Web-application-settings)  
55. formeo/go-audio-converter \- GitHub, [https://github.com/formeo/go-audio-converter](https://github.com/formeo/go-audio-converter)  
56. I tried to capture system audio in the browser. Here's what I learned. \- DEV Community, [https://dev.to/flo152121063061/i-tried-to-capture-system-audio-in-the-browser-heres-what-i-learned-1f99](https://dev.to/flo152121063061/i-tried-to-capture-system-audio-in-the-browser-heres-what-i-learned-1f99)  
57. Web API HTMLMediaElement captureStream() Method \- GeeksforGeeks, [https://www.geeksforgeeks.org/javascript/web-api-htmlmediaelement-capturestream-method/](https://www.geeksforgeeks.org/javascript/web-api-htmlmediaelement-capturestream-method/)  
58. Record audio and video with MediaRecorder | Blog \- Chrome for Developers, [https://developer.chrome.com/blog/mediarecorder](https://developer.chrome.com/blog/mediarecorder)  
59. Using the MediaStream Recording API \- MDN Web Docs, [https://developer.mozilla.org/en-US/docs/Web/API/MediaStream\_Recording\_API/Using\_the\_MediaStream\_Recording\_API](https://developer.mozilla.org/en-US/docs/Web/API/MediaStream_Recording_API/Using_the_MediaStream_Recording_API)  
60. runtime package \- github.com/chromedp/cdproto/runtime \- Go Packages, [https://pkg.go.dev/github.com/chromedp/cdproto/runtime](https://pkg.go.dev/github.com/chromedp/cdproto/runtime)  
61. braheezy/shine-mp3: Pure Go port of Shine MP3 Encoder \- GitHub, [https://github.com/braheezy/shine-mp3](https://github.com/braheezy/shine-mp3)  
62. cdproto/target/target.go at main \- GitHub, [https://github.com/chromedp/cdproto/blob/master/target/target.go](https://github.com/chromedp/cdproto/blob/master/target/target.go)  
63. Activity · Zerg00s/Live-Captions-Saver \- GitHub, [https://github.com/Zerg00s/Live-Captions-Saver/activity](https://github.com/Zerg00s/Live-Captions-Saver/activity)  
64. MS Teams Live Captions Saver Browser Extension v4.8.1 \- GitHub, [https://github.com/Zerg00s/Live-Captions-Saver](https://github.com/Zerg00s/Live-Captions-Saver)  
65. EventBindingCalled Not Triggering · Issue \#1337 \- GitHub, [https://github.com/chromedp/chromedp/issues/1337](https://github.com/chromedp/chromedp/issues/1337)  
66. Teams not syncing/loading after S25 Ultra Update \- Microsoft Learn, [https://learn.microsoft.com/en-us/answers/questions/5864073/teams-not-syncing-loading-after-s25-ultra-update](https://learn.microsoft.com/en-us/answers/questions/5864073/teams-not-syncing-loading-after-s25-ultra-update)  
67. I was invited to a guest organization and I no longer see it in my Teams browser, [https://learn.microsoft.com/en-us/answers/questions/4431458/i-was-invited-to-a-guest-organization-and-i-no-lon](https://learn.microsoft.com/en-us/answers/questions/4431458/i-was-invited-to-a-guest-organization-and-i-no-lon)  
68. Teams Join Online button opens browser in stead of desktop application, [https://techcommunity.microsoft.com/discussions/microsoftteams/teams-join-online-button-opens-browser-in-stead-of-desktop-application/1169997/replies/1172932](https://techcommunity.microsoft.com/discussions/microsoftteams/teams-join-online-button-opens-browser-in-stead-of-desktop-application/1169997/replies/1172932)  
69. Teams Join Online button opens browser in stead of desktop application, [https://techcommunity.microsoft.com/discussions/microsoftteams/teams-join-online-button-opens-browser-in-stead-of-desktop-application/1169997](https://techcommunity.microsoft.com/discussions/microsoftteams/teams-join-online-button-opens-browser-in-stead-of-desktop-application/1169997)