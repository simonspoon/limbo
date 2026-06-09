package commands

import (
	"encoding/json"
	"fmt"

	"github.com/fatih/color"
	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/store/taskstore"
	"github.com/spf13/cobra"
)

var (
	docAddTitle   string
	docAddType    string
	docAddBody    string
	docAddSlug    string
	docAddPretty  bool
	docShowPretty bool
)

// docCmd is the parent of the doc subcommand tree. It carries no RunE.
var docCmd = &cobra.Command{
	Use:   "doc",
	Short: "Manage project-scoped documents (PRD, architecture, ADRs)",
	Long: `Manage project-scoped documents stored in the central store.

Documents (PRDs, architecture overviews, ADRs, ...) live under <root>/docs/.
Each carries a compact index entry and a markdown body sidecar. ADR-type docs
additionally carry a status lifecycle (proposed -> accepted -> superseded).`,
}

var docAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new document",
	Args:  cobra.NoArgs,
	RunE:  runDocAdd,
}

var docListCmd = &cobra.Command{
	Use:   "list",
	Short: "List documents",
	Args:  cobra.NoArgs,
	RunE:  runDocList,
}

var docShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show a document (including its body)",
	Args:  cobra.ExactArgs(1),
	RunE:  runDocShow,
}

var docDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a document",
	Args:  cobra.ExactArgs(1),
	RunE:  runDocDelete,
}

var docLinkCmd = &cobra.Command{
	Use:   "link <doc-id> <task-id>",
	Short: "Link a document to a task (bidirectional)",
	Args:  cobra.ExactArgs(2),
	RunE:  runDocLink,
}

var docStatusCmd = &cobra.Command{
	Use:   "status <doc-id> <new-status>",
	Short: "Transition an ADR's status (proposed -> accepted -> superseded)",
	Args:  cobra.ExactArgs(2),
	RunE:  runDocStatus,
}

var docSupersedeCmd = &cobra.Command{
	Use:   "supersede <old-adr-id> <new-adr-id>",
	Short: "Record that a new ADR supersedes an old one (bidirectional)",
	Args:  cobra.ExactArgs(2),
	RunE:  runDocSupersede,
}

func init() {
	docAddCmd.Flags().StringVar(&docAddTitle, "title", "", "Document title (required)")
	docAddCmd.Flags().StringVar(&docAddType, "type", "", "Document type (e.g. prd, arch, adr)")
	docAddCmd.Flags().StringVar(&docAddBody, "body", "", "Document body (markdown)")
	docAddCmd.Flags().StringVar(&docAddSlug, "slug", "", "Override the derived slug")
	docAddCmd.Flags().BoolVar(&docAddPretty, "pretty", false, "Pretty print output")

	docShowCmd.Flags().BoolVar(&docShowPretty, "pretty", false, "Pretty print output")

	docCmd.AddCommand(docAddCmd)
	docCmd.AddCommand(docListCmd)
	docCmd.AddCommand(docShowCmd)
	docCmd.AddCommand(docDeleteCmd)
	docCmd.AddCommand(docLinkCmd)
	docCmd.AddCommand(docStatusCmd)
	docCmd.AddCommand(docSupersedeCmd)
}

// jsonError writes a {"error": "..."} object to the command's stderr. The
// returned error (from RunE) is what root.go turns into the non-zero exit; the
// JSON object on stderr satisfies the "JSON error" criteria literally.
func jsonError(cmd *cobra.Command, msg string) {
	out, _ := json.Marshal(struct {
		Error string `json:"error"`
	}{msg})
	fmt.Fprintln(cmd.ErrOrStderr(), string(out))
}

// docShowResult is the show wire format: the doc metadata plus its body.
type docShowResult struct {
	models.Doc
	Body string `json:"body"`
}

func runDocAdd(cmd *cobra.Command, args []string) error {
	if docAddTitle == "" {
		jsonError(cmd, "document title required: provide --title")
		return fmt.Errorf("document title required: provide --title")
	}

	store, err := getStorage()
	if err != nil {
		return err
	}

	doc := models.Doc{
		Title: docAddTitle,
		Type:  docAddType,
	}

	stored, err := store.Docs().Add(doc, docAddBody)
	if err != nil {
		jsonError(cmd, err.Error())
		return err
	}

	// Honor an explicit --slug override after the collision-safe generation.
	if docAddSlug != "" {
		stored.Slug = docAddSlug
		if err := store.Docs().Update(stored); err != nil {
			jsonError(cmd, err.Error())
			return err
		}
	}

	if docAddPretty {
		green := color.New(color.FgGreen)
		green.Printf("Created doc %s: %s\n", stored.ID, stored.Title)
	} else {
		fmt.Println(stored.ID)
	}
	return nil
}

func runDocList(cmd *cobra.Command, args []string) error {
	store, err := getStorage()
	if err != nil {
		return err
	}

	docs, err := store.Docs().List()
	if err != nil {
		jsonError(cmd, err.Error())
		return err
	}

	// Always emit a JSON array, even when empty.
	out, err := json.Marshal(docs)
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}

func runDocShow(cmd *cobra.Command, args []string) error {
	id := models.NormalizeDocID(args[0])
	if !models.IsValidDocID(id) {
		jsonError(cmd, fmt.Sprintf("invalid doc ID: %s", args[0]))
		return fmt.Errorf("invalid doc ID: %s", args[0])
	}

	store, err := getStorage()
	if err != nil {
		return err
	}

	doc, err := store.Docs().Get(id)
	if err != nil {
		jsonError(cmd, err.Error())
		return err
	}

	body, err := store.Docs().ReadBody(id)
	if err != nil {
		jsonError(cmd, err.Error())
		return err
	}

	if docShowPretty {
		printDocDetails(&doc, body)
		return nil
	}

	result := docShowResult{Doc: doc, Body: body}
	out, _ := json.Marshal(result)
	fmt.Println(string(out))
	return nil
}

func runDocDelete(cmd *cobra.Command, args []string) error {
	id := models.NormalizeDocID(args[0])
	if !models.IsValidDocID(id) {
		jsonError(cmd, fmt.Sprintf("invalid doc ID: %s", args[0]))
		return fmt.Errorf("invalid doc ID: %s", args[0])
	}

	store, err := getStorage()
	if err != nil {
		return err
	}

	if err := store.Docs().Delete(id); err != nil {
		jsonError(cmd, err.Error())
		return err
	}

	out, _ := json.Marshal(struct {
		ID      string `json:"id"`
		Deleted bool   `json:"deleted"`
	}{id, true})
	fmt.Println(string(out))
	return nil
}

func runDocLink(cmd *cobra.Command, args []string) error {
	docID := models.NormalizeDocID(args[0])
	if !models.IsValidDocID(docID) {
		jsonError(cmd, fmt.Sprintf("invalid doc ID: %s", args[0]))
		return fmt.Errorf("invalid doc ID: %s", args[0])
	}
	taskID := models.NormalizeTaskID(args[1])
	if !models.IsValidTaskID(taskID) {
		jsonError(cmd, fmt.Sprintf("invalid task ID: %s", args[1]))
		return fmt.Errorf("invalid task ID: %s", args[1])
	}

	store, err := getStorage()
	if err != nil {
		return err
	}

	// Fetch the doc (shared lock, released before any task mutation).
	doc, err := store.Docs().Get(docID)
	if err != nil {
		jsonError(cmd, err.Error())
		return err
	}

	// Validate the task exists (shared-lock load, released before mutation).
	task, err := store.LoadTask(taskID)
	if err != nil {
		if err == taskstore.ErrTaskNotFound {
			jsonError(cmd, fmt.Sprintf("task %s not found", taskID))
			return fmt.Errorf("task %s not found", taskID)
		}
		jsonError(cmd, err.Error())
		return err
	}

	// --- Task side: separate, sequential lock acquisition (R2). ---
	if !containsStr(task.LinkedDocs, docID) {
		task.LinkedDocs = append(task.LinkedDocs, docID)
		if err := store.SaveTask(task); err != nil {
			jsonError(cmd, err.Error())
			return err
		}
	}

	// --- Doc side: a fully separate, sequential lock acquisition (R2). ---
	if !containsStr(doc.LinkedTasks, taskID) {
		doc.LinkedTasks = append(doc.LinkedTasks, taskID)
		if err := store.Docs().Update(doc); err != nil {
			jsonError(cmd, err.Error())
			return err
		}
	}

	out, _ := json.Marshal(struct {
		Doc    string `json:"doc"`
		Task   string `json:"task"`
		Linked bool   `json:"linked"`
	}{docID, taskID, true})
	fmt.Println(string(out))
	return nil
}

func runDocStatus(cmd *cobra.Command, args []string) error {
	id := models.NormalizeDocID(args[0])
	if !models.IsValidDocID(id) {
		jsonError(cmd, fmt.Sprintf("invalid doc ID: %s", args[0]))
		return fmt.Errorf("invalid doc ID: %s", args[0])
	}
	newStatus := args[1]

	store, err := getStorage()
	if err != nil {
		return err
	}

	doc, err := store.Docs().Get(id)
	if err != nil {
		jsonError(cmd, err.Error())
		return err
	}

	if !models.IsADR(&doc) {
		msg := fmt.Sprintf("doc %s is not an ADR; status lifecycle applies only to ADRs", id)
		jsonError(cmd, msg)
		return fmt.Errorf("%s", msg)
	}

	if !models.IsValidADRTransition(doc.Status, newStatus) {
		msg := fmt.Sprintf("invalid ADR status transition: %s -> %s", doc.Status, newStatus)
		jsonError(cmd, msg)
		return fmt.Errorf("%s", msg)
	}

	doc.Status = newStatus
	if err := store.Docs().Update(doc); err != nil {
		jsonError(cmd, err.Error())
		return err
	}

	out, _ := json.Marshal(struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}{id, newStatus})
	fmt.Println(string(out))
	return nil
}

func runDocSupersede(cmd *cobra.Command, args []string) error {
	oldID := models.NormalizeDocID(args[0])
	if !models.IsValidDocID(oldID) {
		jsonError(cmd, fmt.Sprintf("invalid doc ID: %s", args[0]))
		return fmt.Errorf("invalid doc ID: %s", args[0])
	}
	newID := models.NormalizeDocID(args[1])
	if !models.IsValidDocID(newID) {
		jsonError(cmd, fmt.Sprintf("invalid doc ID: %s", args[1]))
		return fmt.Errorf("invalid doc ID: %s", args[1])
	}
	if oldID == newID {
		msg := "cannot supersede a doc with itself"
		jsonError(cmd, msg)
		return fmt.Errorf("%s", msg)
	}

	store, err := getStorage()
	if err != nil {
		return err
	}

	oldDoc, err := store.Docs().Get(oldID)
	if err != nil {
		jsonError(cmd, err.Error())
		return err
	}
	newDoc, err := store.Docs().Get(newID)
	if err != nil {
		jsonError(cmd, err.Error())
		return err
	}

	if !models.IsADR(&oldDoc) {
		msg := fmt.Sprintf("doc %s is not an ADR; supersede applies only to ADRs", oldID)
		jsonError(cmd, msg)
		return fmt.Errorf("%s", msg)
	}
	if !models.IsADR(&newDoc) {
		msg := fmt.Sprintf("doc %s is not an ADR; supersede applies only to ADRs", newID)
		jsonError(cmd, msg)
		return fmt.Errorf("%s", msg)
	}

	oldDoc.SupersededBy = newID
	oldDoc.Status = models.ADRStatusSuperseded
	newDoc.Supersedes = oldID

	// Both ends are staged into a single index persist so the relationship is
	// never left half-formed (R6).
	if err := store.Docs().UpdatePair(oldDoc, newDoc); err != nil {
		jsonError(cmd, err.Error())
		return err
	}

	out, _ := json.Marshal(struct {
		Old        string `json:"old"`
		New        string `json:"new"`
		Superseded bool   `json:"superseded"`
	}{oldID, newID, true})
	fmt.Println(string(out))
	return nil
}

func containsStr(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func printDocDetails(doc *models.Doc, body string) {
	cyan := color.New(color.FgCyan, color.Bold)
	white := color.New(color.FgWhite)
	gray := color.New(color.FgHiBlack)

	white.Printf("ID:          %s\n", doc.ID)
	white.Printf("Slug:        %s\n", doc.Slug)
	white.Printf("Title:       %s\n", doc.Title)
	white.Printf("Type:        %s\n", doc.Type)
	printField(white, "Status", doc.Status)
	printField(white, "Supersedes", doc.Supersedes)
	printField(white, "Superseded", doc.SupersededBy)
	if len(doc.LinkedTasks) > 0 {
		white.Printf("Linked:      %v\n", doc.LinkedTasks)
	}
	gray.Printf("Created:     %s\n", doc.Created.Format("2006-01-02 15:04:05"))
	gray.Printf("Updated:     %s\n", doc.Updated.Format("2006-01-02 15:04:05"))
	if body != "" {
		fmt.Println()
		cyan.Println("Body:")
		fmt.Println(body)
	}
}
