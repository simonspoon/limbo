package commands

import (
	"encoding/json"
	"fmt"

	"github.com/fatih/color"
	"github.com/simonspoon/limbo/internal/storage"
	"github.com/simonspoon/limbo/internal/template"
	"github.com/spf13/cobra"
)

var (
	templatePretty bool
	templateParent string
)

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "Manage task templates",
	Long:  `List, show, and apply reusable task hierarchy templates.`,
}

var templateListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available templates",
	Long:  `List all available templates including built-in and project-local templates.`,
	Args:  cobra.NoArgs,
	RunE:  runTemplateList,
}

var templateShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show a template's task hierarchy",
	Long:  `Display the task hierarchy a template would create without actually creating any tasks.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runTemplateShow,
}

var templateApplyCmd = &cobra.Command{
	Use:   "apply <name>",
	Short: "Apply a template to create tasks",
	Long:  `Apply a template, creating all tasks with parent/child relationships and block dependencies.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runTemplateApply,
}

func init() {
	templateCmd.PersistentFlags().BoolVar(&templatePretty, "pretty", false, "Pretty print output")
	templateApplyCmd.Flags().StringVar(&templateParent, "parent", "", "Parent task ID to nest template under")
	templateCmd.AddCommand(templateListCmd)
	templateCmd.AddCommand(templateShowCmd)
	templateCmd.AddCommand(templateApplyCmd)
}

func runTemplateList(cmd *cobra.Command, args []string) error {
	store, err := storage.NewStorage()
	if err != nil {
		return err
	}

	templates, err := template.ListTemplates(store.GetRootDir())
	if err != nil {
		return err
	}

	if templatePretty {
		if len(templates) == 0 {
			fmt.Println("No templates available.")
			return nil
		}
		bold := color.New(color.Bold)
		dim := color.New(color.FgHiBlack)
		for _, t := range templates {
			bold.Printf("  %s", t.Name)
			dim.Printf("  %s\n", t.Description)
		}
		return nil
	}

	type templateInfo struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	var items []templateInfo
	for _, t := range templates {
		items = append(items, templateInfo{
			Name:        t.Name,
			Description: t.Description,
		})
	}
	out, err := json.Marshal(items)
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}

func runTemplateShow(cmd *cobra.Command, args []string) error {
	store, err := storage.NewStorage()
	if err != nil {
		return err
	}

	tmpl, err := template.GetTemplate(store.GetRootDir(), args[0])
	if err != nil {
		return err
	}

	if templatePretty {
		fmt.Print(template.RenderTree(tmpl))
		return nil
	}

	out, err := json.MarshalIndent(tmpl, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}

func runTemplateApply(cmd *cobra.Command, args []string) error {
	store, err := storage.NewStorage()
	if err != nil {
		return err
	}

	tmpl, err := template.GetTemplate(store.GetRootDir(), args[0])
	if err != nil {
		return err
	}

	result, err := template.Apply(store, tmpl, templateParent)
	if err != nil {
		return err
	}

	if templatePretty {
		green := color.New(color.FgGreen)
		green.Printf("Applied template %q: created %d tasks\n", tmpl.Name, len(result.CreatedIDs))
		for _, id := range result.CreatedIDs {
			fmt.Printf("  %s\n", id)
		}
		return nil
	}

	out, err := json.Marshal(result)
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}
