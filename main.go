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
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"github.com/wizardsord/thai-smartcard-reader/pkg/model"
	"github.com/wizardsord/thai-smartcard-reader/pkg/server"
	"github.com/wizardsord/thai-smartcard-reader/pkg/smc"
	"github.com/wizardsord/thai-smartcard-reader/pkg/util"
)

func main() {
	a := app.NewWithID("com.jittaconnext.smartcard-reader")
	iconResource := resourceIconPng
	a.SetIcon(iconResource)

	w := a.NewWindow("Smart Card Reader")
	w.Resize(fyne.NewSize(800, 600))

	logBinding := binding.NewString()
	logBinding.Set("Smart Card Reader Logs\n")

	logArea := widget.NewMultiLineEntry()
	logArea.Bind(logBinding)
	logArea.Disable()

	scrollContainer := container.NewScroll(logArea)

	var logMutex sync.Mutex
	log.SetOutput(&LogWriter{
		logBinding: logBinding,
		mutex:      &logMutex,
	})

	go startSmartCardReader(&logMutex)

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

	w.SetCloseIntercept(func() {
		w.Hide()
	})

	w.SetContent(container.NewBorder(nil, widget.NewButton("Exit", func() {
		a.Quit()
	}), nil, nil, scrollContainer))

	w.ShowAndRun()
}

// LogWriter appends log text safely via binding.String
type LogWriter struct {
	logBinding binding.String
	mutex      *sync.Mutex
}

func (lw *LogWriter) Write(p []byte) (n int, err error) {
	lw.mutex.Lock()
	defer lw.mutex.Unlock()

	current, _ := lw.logBinding.Get()
	err = lw.logBinding.Set(current + string(p))
	return len(p), err
}

func startSmartCardReader(logMutex *sync.Mutex) {
	port := util.GetEnv("SMC_AGENT_PORT", "9898")
	showImage := util.GetEnvBool("SMC_SHOW_IMAGE", true)
	showLaser := util.GetEnvBool("SMC_SHOW_LASER", true)
	showNhso := util.GetEnvBool("SMC_SHOW_NHSO", false)

	broadcast := make(chan model.Message)

	serverCfg := server.ServerConfig{
		Broadcast: broadcast,
		Port:      port,
	}
	go func() {
		log.Printf("Starting server on port: %s\n", port)
		server.Serve(serverCfg)
	}()

	opts := &smc.Options{
		ShowFaceImage: showImage,
		ShowNhsoData:  showNhso,
		ShowLaserData: showLaser,
	}

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

	go func() {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt)
		s := <-signalChan
		log.Printf("Received signal %v. Shutting down gracefully...\n", s)
	}()
}
