package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"git.sr.ht/~rockorager/go-jmap"
	"github.com/spf13/cobra"
	"github.com/vicyap/fastmail-cli/internal/client"
	"github.com/vicyap/fastmail-cli/internal/output"
	"github.com/vicyap/fastmail-cli/internal/sieve"
)

var sieveCmd = &cobra.Command{
	Use:   "sieve",
	Short: "Manage Sieve scripts",
}

var sieveListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all Sieve scripts",
	RunE:  runSieveList,
}

var sieveGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a Sieve script's content",
	Args:  cobra.ExactArgs(1),
	RunE:  runSieveGet,
}

var sieveSetCmd = &cobra.Command{
	Use:   "set <name> [file]",
	Short: "Upload a Sieve script (reads stdin if no file given)",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runSieveSet,
}

var sieveActivateCmd = &cobra.Command{
	Use:   "activate <id>",
	Short: "Activate a Sieve script",
	Args:  cobra.ExactArgs(1),
	RunE:  runSieveActivate,
}

var sieveDeactivateCmd = &cobra.Command{
	Use:   "deactivate",
	Short: "Deactivate the current active Sieve script",
	RunE:  runSieveDeactivate,
}

var sieveDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a Sieve script",
	Args:  cobra.ExactArgs(1),
	RunE:  runSieveDelete,
}

var sieveValidateCmd = &cobra.Command{
	Use:   "validate [file]",
	Short: "Validate a Sieve script without storing it (reads stdin if no file given)",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runSieveValidate,
}

func init() {
	rootCmd.AddCommand(sieveCmd)
	sieveCmd.AddCommand(sieveListCmd)
	sieveCmd.AddCommand(sieveGetCmd)
	sieveCmd.AddCommand(sieveSetCmd)
	sieveCmd.AddCommand(sieveActivateCmd)
	sieveCmd.AddCommand(sieveDeactivateCmd)
	sieveCmd.AddCommand(sieveDeleteCmd)
	sieveCmd.AddCommand(sieveValidateCmd)
}

func sieveAccountID(c *client.Client) jmap.ID {
	// Sieve might use the mail account
	id := c.JMAP.Session.PrimaryAccounts[sieve.URI]
	if id == "" {
		return c.MailAccountID()
	}
	return id
}

func runSieveList(cmd *cobra.Command, args []string) error {
	c, err := client.New(tokenFlag)
	if err != nil {
		return err
	}

	req := &jmap.Request{}
	req.Invoke(&sieve.Get{
		Account: sieveAccountID(c),
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return fmt.Errorf("sieve list failed: %w", err)
	}

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return fmt.Errorf("JMAP error: %s", r.Error())
		case *sieve.GetResponse:
			return printSieveScripts(r.List)
		}
	}

	return fmt.Errorf("unexpected response from server")
}

func runSieveGet(cmd *cobra.Command, args []string) error {
	c, err := client.New(tokenFlag)
	if err != nil {
		return err
	}

	scriptID := jmap.ID(args[0])

	// Get script metadata to find blobId
	req := &jmap.Request{}
	req.Invoke(&sieve.Get{
		Account: sieveAccountID(c),
		IDs:     []jmap.ID{scriptID},
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return fmt.Errorf("sieve get failed: %w", err)
	}

	var blobID jmap.ID
	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return fmt.Errorf("JMAP error: %s", r.Error())
		case *sieve.GetResponse:
			if len(r.List) == 0 {
				return fmt.Errorf("script not found: %s", scriptID)
			}
			if jsonOutput {
				return output.PrintJSON(r.List[0])
			}
			blobID = r.List[0].BlobID
		}
	}

	if blobID == "" {
		return fmt.Errorf("script has no blob ID")
	}

	// Download the script content
	reader, err := c.JMAP.Download(sieveAccountID(c), blobID)
	if err != nil {
		return fmt.Errorf("failed to download script: %w", err)
	}
	defer reader.Close()

	_, err = io.Copy(os.Stdout, reader)
	return err
}

func runSieveSet(cmd *cobra.Command, args []string) error {
	c, err := client.New(tokenFlag)
	if err != nil {
		return err
	}

	name := args[0]

	// Read script content
	var scriptContent io.Reader
	if len(args) > 1 {
		file, err := os.Open(args[1])
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()
		scriptContent = file
	} else {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read stdin: %w", err)
		}
		scriptContent = bytes.NewReader(data)
	}

	accountID := sieveAccountID(c)

	// Upload the script as a blob
	uploadResp, err := c.JMAP.Upload(accountID, scriptContent)
	if err != nil {
		return fmt.Errorf("failed to upload script: %w", err)
	}

	// Create the script
	nameStr := name
	req := &jmap.Request{}
	req.Invoke(&sieve.Set{
		Account: accountID,
		Create: map[jmap.ID]*sieve.SieveScript{
			"new1": {
				Name:   &nameStr,
				BlobID: uploadResp.ID,
			},
		},
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return fmt.Errorf("sieve set failed: %w", err)
	}

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return fmt.Errorf("JMAP error: %s", r.Error())
		case *sieve.SetResponse:
			if r.NotCreated != nil {
				if setErr, ok := r.NotCreated["new1"]; ok {
					return fmt.Errorf("failed to create script: %s", formatSieveSetError(setErr))
				}
			}
			created := r.Created["new1"]
			if jsonOutput {
				return output.PrintJSON(created)
			}
			fmt.Fprintf(os.Stderr, "Script '%s' created (ID: %s)\n", name, created.ID)
			return nil
		}
	}

	return fmt.Errorf("unexpected response from server")
}

func runSieveActivate(cmd *cobra.Command, args []string) error {
	c, err := client.New(tokenFlag)
	if err != nil {
		return err
	}

	scriptID := jmap.ID(args[0])

	req := &jmap.Request{}
	req.Invoke(&sieve.Set{
		Account:                 sieveAccountID(c),
		OnSuccessActivateScript: &scriptID,
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return fmt.Errorf("sieve activate failed: %w", err)
	}

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return fmt.Errorf("JMAP error: %s", r.Error())
		case *sieve.SetResponse:
			if jsonOutput {
				return output.PrintJSON(map[string]string{"id": string(scriptID), "action": "activated"})
			}
			fmt.Fprintf(os.Stderr, "Script %s activated.\n", scriptID)
			return nil
		}
	}

	return fmt.Errorf("unexpected response from server")
}

func runSieveDeactivate(cmd *cobra.Command, args []string) error {
	c, err := client.New(tokenFlag)
	if err != nil {
		return err
	}

	deactivate := true
	req := &jmap.Request{}
	req.Invoke(&sieve.Set{
		Account:                   sieveAccountID(c),
		OnSuccessDeactivateScript: &deactivate,
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return fmt.Errorf("sieve deactivate failed: %w", err)
	}

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return fmt.Errorf("JMAP error: %s", r.Error())
		case *sieve.SetResponse:
			if jsonOutput {
				return output.PrintJSON(map[string]string{"action": "deactivated"})
			}
			fmt.Fprintln(os.Stderr, "Active script deactivated.")
			return nil
		}
	}

	return fmt.Errorf("unexpected response from server")
}

func runSieveDelete(cmd *cobra.Command, args []string) error {
	c, err := client.New(tokenFlag)
	if err != nil {
		return err
	}

	scriptID := jmap.ID(args[0])

	req := &jmap.Request{}
	req.Invoke(&sieve.Set{
		Account: sieveAccountID(c),
		Destroy: []jmap.ID{scriptID},
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return fmt.Errorf("sieve delete failed: %w", err)
	}

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return fmt.Errorf("JMAP error: %s", r.Error())
		case *sieve.SetResponse:
			if r.NotDestroyed != nil {
				if setErr, ok := r.NotDestroyed[scriptID]; ok {
					return fmt.Errorf("failed to delete script: %s", formatSieveSetError(setErr))
				}
			}
			if jsonOutput {
				return output.PrintJSON(map[string]string{"id": string(scriptID), "action": "deleted"})
			}
			fmt.Fprintf(os.Stderr, "Script %s deleted.\n", scriptID)
			return nil
		}
	}

	return fmt.Errorf("unexpected response from server")
}

func runSieveValidate(cmd *cobra.Command, args []string) error {
	c, err := client.New(tokenFlag)
	if err != nil {
		return err
	}

	var scriptContent io.Reader
	if len(args) > 0 {
		file, err := os.Open(args[0])
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()
		scriptContent = file
	} else {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read stdin: %w", err)
		}
		scriptContent = bytes.NewReader(data)
	}

	accountID := sieveAccountID(c)

	// Upload the script as a blob
	uploadResp, err := c.JMAP.Upload(accountID, scriptContent)
	if err != nil {
		return fmt.Errorf("failed to upload script: %w", err)
	}

	req := &jmap.Request{}
	req.Invoke(&sieve.Validate{
		Account: accountID,
		BlobID:  uploadResp.ID,
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return fmt.Errorf("sieve validate failed: %w", err)
	}

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return fmt.Errorf("JMAP error: %s", r.Error())
		case *sieve.ValidateResponse:
			if r.Error != nil {
				if jsonOutput {
					return output.PrintJSON(r.Error)
				}
				return fmt.Errorf("script validation failed: %s", formatSieveSetError(r.Error))
			}
			if jsonOutput {
				return output.PrintJSON(map[string]string{"status": "valid"})
			}
			fmt.Fprintln(os.Stderr, "Script is valid.")
			return nil
		}
	}

	return fmt.Errorf("unexpected response from server")
}

func printSieveScripts(scripts []*sieve.SieveScript) error {
	if jsonOutput {
		return output.PrintJSON(scripts)
	}

	tbl := output.NewTable(os.Stdout)
	tbl.Headers("ID", "NAME", "ACTIVE")
	for _, s := range scripts {
		name := "(unnamed)"
		if s.Name != nil {
			name = *s.Name
		}
		active := "no"
		if s.IsActive {
			active = "yes"
		}
		tbl.Row(string(s.ID), name, active)
	}
	return tbl.Flush()
}

func formatSieveSetError(e *jmap.SetError) string {
	if e.Description != nil {
		return fmt.Sprintf("%s: %s", e.Type, *e.Description)
	}
	return e.Type
}
