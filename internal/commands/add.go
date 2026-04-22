package commands

import (
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/simonspoon/limbo/internal/models"
	"github.com/spf13/cobra"
)

var (
	addName               string
	addDescription        string
	addParent             string
	addPretty             bool
	addApproach           string
	addAction             string
	addVerify             string
	addResult             string
	addAcceptanceCriteria string
	addScopeOut           string
	addAffectedAreas      string
	addTestStrategy       string
	addRisks              string
	addReport             string
)

var addCmd = &cobra.Command{
	Use:   "add [name]",
	Short: "Add a new task",
	Long:  `Add a new task with the specified name and optional description. Name may be given as a positional argument or via --name.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runAdd,
}

func init() {
	addCmd.Flags().StringVar(&addName, "name", "", "Task name (alternative to positional arg; positional wins if both given)")
	addCmd.Flags().StringVarP(&addDescription, "description", "d", "", "Task description")
	addCmd.Flags().StringVar(&addParent, "parent", "", "Parent task ID")
	addCmd.Flags().BoolVar(&addPretty, "pretty", false, "Pretty print output")
	addCmd.Flags().StringVar(&addApproach, "approach", "", "What concrete work to perform")
	addCmd.Flags().StringVar(&addAction, "action", "", "What concrete work to perform (alias for --approach)")
	addCmd.Flags().StringVar(&addVerify, "verify", "", "How to confirm the action succeeded")
	addCmd.Flags().StringVar(&addResult, "result", "", "Template for what to report back")
	addCmd.Flags().StringVar(&addAcceptanceCriteria, "acceptance-criteria", "", "Criteria for acceptance")
	addCmd.Flags().StringVar(&addScopeOut, "scope-out", "", "What is explicitly out of scope")
	addCmd.Flags().StringVar(&addAffectedAreas, "affected-areas", "", "Areas of the codebase affected")
	addCmd.Flags().StringVar(&addTestStrategy, "test-strategy", "", "How to test the changes")
	addCmd.Flags().StringVar(&addRisks, "risks", "", "Known risks and mitigations")
	addCmd.Flags().StringVar(&addReport, "report", "", "Completion report")
	_ = addCmd.Flags().MarkHidden("action")
}

func runAdd(cmd *cobra.Command, args []string) error {
	// Resolve task name: positional wins over --name
	var name string
	if len(args) > 0 {
		name = args[0]
	} else {
		name = addName
	}
	if name == "" {
		return fmt.Errorf("task name required: provide positional arg or --name")
	}

	// Load storage
	store, err := getStorage()
	if err != nil {
		return err
	}

	// Validate parent if specified
	var parent *string
	if addParent != "" {
		normalizedParent := models.NormalizeTaskID(addParent)
		if !models.IsValidTaskID(normalizedParent) {
			return fmt.Errorf("invalid parent task ID: %s", addParent)
		}
		parentTask, err := store.LoadTask(normalizedParent)
		if err != nil {
			return fmt.Errorf("parent task %s not found", addParent)
		}
		if parentTask.Status == models.StatusDone {
			return fmt.Errorf("cannot add child to done task")
		}
		parent = &normalizedParent
	}

	// Generate new task ID
	taskID, err := store.GenerateTaskID()
	if err != nil {
		return err
	}

	// Resolve approach: --approach wins, --action is fallback alias
	approach := addApproach
	if approach == "" && addAction != "" {
		approach = addAction
	}

	// Create task
	now := time.Now()
	task := &models.Task{
		ID:                 taskID,
		Name:               name,
		Description:        addDescription,
		Approach:           approach,
		Verify:             addVerify,
		Result:             addResult,
		AcceptanceCriteria: addAcceptanceCriteria,
		ScopeOut:           addScopeOut,
		AffectedAreas:      addAffectedAreas,
		TestStrategy:       addTestStrategy,
		Risks:              addRisks,
		Report:             addReport,
		Parent:             parent,
		Status:             models.StatusCaptured,
		Created:            now,
		Updated:            now,
	}

	// Save task
	if err := store.SaveTask(task); err != nil {
		return err
	}

	if addPretty {
		green := color.New(color.FgGreen)
		green.Printf("Created task %s: %s\n", task.ID, task.Name)
	} else {
		fmt.Println(task.ID)
	}

	return nil
}
