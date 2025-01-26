package main

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"github.com/somprasongd/go-thai-smartcard/pkg/model"
	"github.com/somprasongd/go-thai-smartcard/pkg/server"
	"github.com/somprasongd/go-thai-smartcard/pkg/smc"
	"github.com/somprasongd/go-thai-smartcard/pkg/util"
)

func main() {
	// Create Fyne application
	a := app.NewWithID("com.example.smartcard-reader")

	// Load the custom icon
	iconResource := resourceIconPng

	// Set the application icon
	a.SetIcon(iconResource)

	// Create the main application window
	w := a.NewWindow("Smart Card Reader")
	w.Resize(fyne.NewSize(800, 600))

	// Create the log area
	logArea := widget.NewMultiLineEntry()
	logArea.Disable() // Make the log area effectively read-only
	logArea.SetText("Smart Card Reader Logs\n")

	// Create a scroll container for the log area
	scrollContainer := container.NewScroll(logArea)

	// Mutex to handle concurrent log updates
	var logMutex sync.Mutex

	// Redirect log output to the GUI
	log.SetOutput(&LogWriter{logArea: logArea, scrollContainer: scrollContainer, mutex: &logMutex})

	// Automatically start the smart card reader
	go startSmartCardReader(logArea, &logMutex)

	// Set up System Tray Menu (if desktop application)
	if desk, ok := a.(desktop.App); ok {
		m := fyne.NewMenu("Smart Card Reader",
			fyne.NewMenuItem("Show", func() {
				w.Show()
			}),
			fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem("Exit", func() {
				a.Quit()
			}),
		)
		desk.SetSystemTrayMenu(m)
		desk.SetSystemTrayIcon(iconResource)
	}

	// Intercept the close action to hide the window instead of exiting the app
	w.SetCloseIntercept(func() {
		w.Hide()
	})

	// GUI layout: Log area fills the entire window
	w.SetContent(container.NewBorder(nil, widget.NewButton("Exit", func() {
		a.Quit()
	}), nil, nil, scrollContainer))

	// Run the Fyne application
	w.ShowAndRun()
}

// LogWriter redirects log messages to the Fyne GUI
type LogWriter struct {
	logArea         *widget.Entry
	scrollContainer *container.Scroll
	mutex           *sync.Mutex
}

func (lw *LogWriter) Write(p []byte) (n int, err error) {
	lw.mutex.Lock()
	defer lw.mutex.Unlock()

	// Append log messages to the GUI log area
	lw.logArea.SetText(lw.logArea.Text + string(p))

	// Scroll to the bottom of the log area
	lw.scrollContainer.ScrollToBottom()

	return len(p), nil
}

func startSmartCardReader(logArea *widget.Entry, logMutex *sync.Mutex) {
	// Load environment variables
	port := util.GetEnv("SMC_AGENT_PORT", "9898")
	showImage := util.GetEnvBool("SMC_SHOW_IMAGE", true)
	showLaser := util.GetEnvBool("SMC_SHOW_LASER", true)
	showNhso := util.GetEnvBool("SMC_SHOW_NHSO", false)

	broadcast := make(chan model.Message)

	// Start the server
	serverCfg := server.ServerConfig{
		Broadcast: broadcast,
		Port:      port,
	}
	go func() {
		log.Printf("Starting server on port: %s\n", port)
		server.Serve(serverCfg)
	}()

	// Smart card options
	opts := &smc.Options{
		ShowFaceImage: showImage,
		ShowNhsoData:  showNhso,
		ShowLaserData: showLaser,
	}

	// Start the daemon
	go func() {
		smartCard := smc.NewSmartCard()
		for {
			err := smartCard.StartDaemon(broadcast, opts)
			if err != nil {
				log.Printf("Error in daemon: %v. Retrying...\n", err)
				time.Sleep(2 * time.Second)
			}
		}
	}()

	// Graceful shutdown on signal
	go func() {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt)
		s := <-signalChan
		log.Printf("Received signal %v. Shutting down gracefully...\n", s)
	}()
}
