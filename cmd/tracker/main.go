package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/atakang7/axon"
)

func main() {
	sess := axon.LoadOrCreateSession()
	path := sess.Path()

	for {
		b, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				cmd := exec.Command("clear")
				cmd.Stdout = os.Stdout
				cmd.Run()
				fmt.Printf("================ AXON SESSION TRACKER ================\n")
				fmt.Printf("Waiting for session file to be created...\n")
				fmt.Printf("File: %s\n", path)
				fmt.Printf("(Run 'cortex' or 'axon' in this directory to start a session)\n")
				fmt.Printf("======================================================\n")
			} else {
				fmt.Printf("Error reading %s: %v\n", path, err)
			}
		} else {
			var state axon.Session
			if err := json.Unmarshal(b, &state); err != nil {
				fmt.Printf("Error unmarshaling %s: %v\n", path, err)
			} else {
				cmd := exec.Command("clear")
				cmd.Stdout = os.Stdout
				cmd.Run()

				fmt.Printf("================ AXON SESSION TRACKER ================\n")
				fmt.Printf("File: %s\n", path)
				fmt.Printf("Turn: %d | Total Edits: %d\n", state.Turn, len(state.Edits))

				fmt.Printf("\n--- TASK PLAN ---\n")
				if state.CurrentTask != nil && state.CurrentTask.Goal != "" {
					fmt.Printf("Objective: %s\n", state.CurrentTask.Goal)
					for i, s := range state.CurrentTask.Steps {
						status := "[ ]"
						if s.Done {
							status = "[X]"
						} else if i == state.CurrentTask.CurrentStep {
							status = "[>]"
						}
						fmt.Printf("  %s %d. %s\n", status, i+1, s.Description)
					}
				} else {
					fmt.Println("No active task.")
				}

				fmt.Printf("\n--- FILE EDITS (UNDO STACK) ---\n")
				if len(state.Edits) > 0 {
					for i, e := range state.Edits {
						fmt.Printf("  %d. [REVERT SIZE: %d bytes] %s\n", i, len(e.Before), e.Path)
					}
				} else {
					fmt.Println("No edits recorded.")
				}

				fmt.Printf("\n--- CONVERSATION LOG (%d messages) ---\n", len(state.Messages))
				for i, m := range state.Messages {
					prefix := fmt.Sprintf("[%d] %s", i, strings.ToUpper(string(m.Role)))
					if m.ToolName != "" {
						prefix += fmt.Sprintf(" (%s)", m.ToolName)
					}

					// Truncate content for display
					content := m.Content
					if len(content) > 80 {
						content = content[:77] + "..."
					}

					content = strings.ReplaceAll(content, "\n", "\\n")

					if len(m.ToolCalls) > 0 {
						tools := []string{}
						for _, tc := range m.ToolCalls {
							tools = append(tools, tc.Function.Name)
						}
						fmt.Printf("%-30s | ToolCalls: %v\n", prefix, tools)
					} else {
						fmt.Printf("%-30s | %s\n", prefix, content)
					}
				}

				fmt.Printf("\n======================================================\n")
				fmt.Printf("Auto-refreshing every 1 second (Ctrl+C to quit)...\n")
			}
		}
		time.Sleep(1 * time.Second)
	}
}
