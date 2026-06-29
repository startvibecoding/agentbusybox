//go:build windows

package shell

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"strings"
)

func runInteractivePlatform() int {
	loadHistory()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		for range sigChan {
			// Consume signal so the shell is not killed
		}
	}()

	reader := bufio.NewReader(os.Stdin)
	for {
		dir, _ := os.Getwd()
		if len(dir) > 30 {
			dir = "..." + dir[len(dir)-27:]
		}
		fmt.Printf("%s$ ", dir)

		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line == "exit" {
			break
		}
		commandHistory = append(commandHistory, line)
		saveHistoryLine(line)
		executeLine(line)
	}
	return 0
}
