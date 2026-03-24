package commands

import (
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/simonspoon/limbo/internal/models"
	"github.com/spf13/cobra"
)

var (
	addDescription string
	addParent      string
	addPretty      bool
	addAction      string
	addVerify      string
	addResult      string
)

var addCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new task",
	Long:  `Add a new task with the specified name and optional description.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runAdd,
}

func init() {
	addCmd.Flags().StringVarP(&addDescription, "description", "d", "", "Task description")
	addCmd.Flags().StringVar(&addParent, "parent", "", "Parent task ID")
	addCmd.Flags().BoolVar(&addPretty, "pretty", false, "Pretty print output")
	addCmd.Flags().StringVar(&addAction, "action", "", "What concrete work to perform")
	addCmd.Flags().StringVar(&addVerify, "verify", "", "How to confirm the action succeeded")
	addCmd.Flags().StringVar(&addResult, "result", "", "Template for what to report back")
	if err := addCmd.MarkFlagRequired("action"); err != nil {
		panic(err)
	}
	if err := addCmd.MarkFlagRequired("verify"); err != nil {
		panic(err)
	}
	if err := addCmd.MarkFlagRequired("result"); err != nil {
		panic(err)
	}
}

func runAdd(cmd *cobra.Command, args []string) error {
	// Get task name
	name := args[0]

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

	// Create task
	now := time.Now()
	task := &models.Task{
		ID:          taskID,
		Name:        name,
		Description: addDescription,
		Action:      addAction,
		Verify:      addVerify,
		Result:      addResult,
		Parent:      parent,
		Status:      models.StatusTodo,
		Created:     now,
		Updated:     now,
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
