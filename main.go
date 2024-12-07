package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/go-co-op/gocron/v2"
)

func main() {
	s, err := gocron.NewScheduler()
	if err != nil {
		log.Fatal(err)
	}
	s.Start()
	defer func() { _ = s.Shutdown() }()

	// Create a new watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	// Directory to watch
	watchDir := "/mnt/data/nzbs"

	// File extension to watch for
	watchExt := ".nzb"

	// Start watching the directory
	err = watcher.Add(watchDir)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Watching directory %s for files with extension %s\n", watchDir, watchExt)

	paths := make(chan string, 100)

	go func() {
		for newPath := range paths {
			now := time.Now()
			job, err := s.NewJob(
				gocron.WeeklyJob(
					1,
					gocron.NewWeekdays(now.Weekday()),
					gocron.NewAtTimes(
						gocron.NewAtTime(uint(now.Hour()), uint(now.Minute())-5, 0),
					),
				),
				gocron.NewTask(
					func() { queueNZB(newPath) },
				),
			)
			if err != nil {
				log.Print(fmt.Sprintf("Error: Failed to schedule job for %s.", newPath), err)
			}
			nextRun, _ := job.NextRun()
			log.Printf("New job scheduled: %s @ %s\n", newPath, nextRun)
		}

	}()

	files, err := os.ReadDir(watchDir)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == watchExt {
			paths <- filepath.Join(watchDir, file.Name())
		}
	}

	// Start the file watching loop
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Create == fsnotify.Create {
				if filepath.Ext(event.Name) == watchExt {
					log.Printf("New file created: %s\n", event.Name)

					paths <- event.Name

				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("error:", err)
		}
	}
}

func queueNZB(nzbpath string) {
	conn, err := net.Dial("tcp", "localhost:6666")
	if err != nil {
		log.Println("Error connecting to service:", err)
		return
	}
	defer conn.Close()

	fmt.Fprintf(conn, "%s\n", nzbpath)
	response, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		log.Println("Error reading response:", err)
		return
	}

	log.Print("Response from service: ", response)
}
