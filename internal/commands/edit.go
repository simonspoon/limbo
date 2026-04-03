package commands

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/storage"
	"github.com/spf13/cobra"
)

var (
	editName               string
	editDescription        string
	editApproach           string
	editAction             string
	editVerify             string
	editResult             string
	editAcceptanceCriteria string
	editScopeOut           string
	editAffectedAreas      string
	editTestStrategy       string
	editRisks              string
	editReport             string
	editPretty             bool
)

var editCmd = &cobra.Command{
	Use:   "edit <id>",
	Short: "Edit a task's fields",
	Long:  `Modify an existing task's mutable fields. Only specified flags are updated.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runEdit,
}

func init() {
	editCmd.Flags().StringVar(&editName, "name", "", "Task name")
	editCmd.Flags().StringVarP(&editDescription, "description", "d", "", "Task description")
	editCmd.Flags().StringVar(&editApproach, "approach", "", "What concrete work to perform")
	editCmd.Flags().StringVar(&editAction, "action", "", "What concrete work to perform (alias for --approach)")
	editCmd.Flags().StringVar(&editVerify, "verify", "", "How to confirm the action succeeded")
	editCmd.Flags().StringVar(&editResult, "result", "", "Template for what to report back")
	editCmd.Flags().StringVar(&editAcceptanceCriteria, "acceptance-criteria", "", "Criteria for acceptance")
	editCmd.Flags().StringVar(&editScopeOut, "scope-out", "", "What is explicitly out of scope")
	editCmd.Flags().StringVar(&editAffectedAreas, "affected-areas", "", "Areas of the codebase affected")
	editCmd.Flags().StringVar(&editTestStrategy, "test-strategy", "", "How to test the changes")
	editCmd.Flags().StringVar(&editRisks, "risks", "", "Known risks and mitigations")
	editCmd.Flags().StringVar(&editReport, "report", "", "Completion report")
	editCmd.Flags().BoolVar(&editPretty, "pretty", false, "Pretty print output")
	_ = editCmd.Flags().MarkHidden("action")
}

// applyEditFlags applies changed flags to the task. Returns true if any field was changed.
func applyEditFlags(cmd *cobra.Command, task *models.Task) bool {
	changed := false
	if cmd.Flags().Changed("name") {
		task.Name = editName
		changed = true
	}
	if cmd.Flags().Changed("description") {
		task.Description = editDescription
		changed = true
	}
	if cmd.Flags().Changed("approach") {
		task.Approach = editApproach
		changed = true
	} else if cmd.Flags().Changed("action") {
		task.Approach = editAction
		changed = true
	}
	if cmd.Flags().Changed("verify") {
		task.Verify = editVerify
		changed = true
	}
	if cmd.Flags().Changed("result") {
		task.Result = editResult
		changed = true
	}
	if cmd.Flags().Changed("acceptance-criteria") {
		task.AcceptanceCriteria = editAcceptanceCriteria
		changed = true
	}
	if cmd.Flags().Changed("scope-out") {
		task.ScopeOut = editScopeOut
		changed = true
	}
	if cmd.Flags().Changed("affected-areas") {
		task.AffectedAreas = editAffectedAreas
		changed = true
	}
	if cmd.Flags().Changed("test-strategy") {
		task.TestStrategy = editTestStrategy
		changed = true
	}
	if cmd.Flags().Changed("risks") {
		task.Risks = editRisks
		changed = true
	}
	if cmd.Flags().Changed("report") {
		task.Report = editReport
		changed = true
	}
	return changed
}

func runEdit(cmd *cobra.Command, args []string) error {
	id := models.NormalizeTaskID(args[0])
	if !models.IsValidTaskID(id) {
		return fmt.Errorf("invalid task ID: %s", args[0])
	}

	store, err := getStorage()
	if err != nil {
		return err
	}

	task, err := store.LoadTask(id)
	if err != nil {
		if err == storage.ErrTaskNotFound {
			return fmt.Errorf("task %s not found", id)
		}
		return err
	}

	if !applyEditFlags(cmd, task) {
		return fmt.Errorf("nothing to edit: specify at least one flag (--name, --description, --approach, --verify, --result, --acceptance-criteria, --scope-out, --affected-areas, --test-strategy, --risks, --report)")
	}

	task.Updated = time.Now()

	if err := store.SaveTask(task); err != nil {
		return err
	}

	if editPretty {
		// Load blocker info for complete pretty output
		allTasks, loadErr := store.LoadAll()
		var blockers, blocks []blockerInfo
		if loadErr == nil {
			for _, blockerID := range task.BlockedBy {
				if info := findBlockerInfo(allTasks, blockerID); info != nil {
					blockers = append(blockers, *info)
				}
			}
			for i := range allTasks {
				for _, depID := range allTasks[i].BlockedBy {
					if depID == id {
						blocks = append(blocks, blockerInfo{
							ID:     allTasks[i].ID,
							Name:   allTasks[i].Name,
							Status: allTasks[i].Status,
						})
						break
					}
				}
			}
		}
		green := color.New(color.FgGreen)
		green.Printf("Task %s updated\n", id)
		printTaskDetails(task, blockers, blocks)
	} else {
		out, _ := json.Marshal(struct {
			ID      string `json:"id"`
			Updated string `json:"updated"`
		}{task.ID, task.Updated.Format(time.RFC3339Nano)})
		fmt.Println(string(out))
	}

	return nil
}
