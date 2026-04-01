package cmd

import (
	"fmt"
	"os"
	"time"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail/vacationresponse"
	"github.com/spf13/cobra"
	"github.com/vicyap/fastmail-cli/internal/client"
	"github.com/vicyap/fastmail-cli/internal/output"
)

var (
	vacationSubject  string
	vacationBody     string
	vacationHTMLBody string
	vacationFrom     string
	vacationTo       string
)

var vacationCmd = &cobra.Command{
	Use:   "vacation",
	Short: "Manage vacation auto-reply",
}

var vacationGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Show current vacation auto-reply settings",
	RunE:  runVacationGet,
}

var vacationSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Enable vacation auto-reply",
	RunE:  runVacationSet,
}

var vacationDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable vacation auto-reply",
	RunE:  runVacationDisable,
}

func init() {
	rootCmd.AddCommand(vacationCmd)
	vacationCmd.AddCommand(vacationGetCmd)
	vacationCmd.AddCommand(vacationSetCmd)
	vacationCmd.AddCommand(vacationDisableCmd)

	vacationSetCmd.Flags().StringVar(&vacationSubject, "subject", "", "Auto-reply subject")
	vacationSetCmd.Flags().StringVar(&vacationBody, "body", "", "Auto-reply plain text body")
	vacationSetCmd.Flags().StringVar(&vacationHTMLBody, "html-body", "", "Auto-reply HTML body")
	vacationSetCmd.Flags().StringVar(&vacationFrom, "from", "", "Start date (YYYY-MM-DD)")
	vacationSetCmd.Flags().StringVar(&vacationTo, "to", "", "End date (YYYY-MM-DD)")
}

func vacationAccountID(c *client.Client) jmap.ID {
	id := c.JMAP.Session.PrimaryAccounts[vacationresponse.URI]
	if id == "" {
		return c.MailAccountID()
	}
	return id
}

func runVacationGet(cmd *cobra.Command, args []string) error {
	c, err := client.New(tokenFlag)
	if err != nil {
		return err
	}

	req := &jmap.Request{}
	req.Invoke(&vacationresponse.Get{
		Account: vacationAccountID(c),
		IDs:     []jmap.ID{"singleton"},
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return fmt.Errorf("vacation get failed: %w", err)
	}

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return fmt.Errorf("JMAP error: %s", r.Error())
		case *vacationresponse.GetResponse:
			if len(r.List) == 0 {
				fmt.Fprintln(os.Stderr, "No vacation response configured.")
				return nil
			}
			return printVacation(r.List[0])
		}
	}

	return fmt.Errorf("unexpected response from server")
}

func runVacationSet(cmd *cobra.Command, args []string) error {
	c, err := client.New(tokenFlag)
	if err != nil {
		return err
	}

	patch := jmap.Patch{
		"isEnabled": true,
	}

	if vacationSubject != "" {
		patch["subject"] = vacationSubject
	}
	if vacationBody != "" {
		patch["textBody"] = vacationBody
	}
	if vacationHTMLBody != "" {
		patch["htmlBody"] = vacationHTMLBody
	}
	if vacationFrom != "" {
		t, err := time.Parse("2006-01-02", vacationFrom)
		if err != nil {
			return fmt.Errorf("invalid --from date: %w", err)
		}
		patch["fromDate"] = t.Format(time.RFC3339)
	}
	if vacationTo != "" {
		t, err := time.Parse("2006-01-02", vacationTo)
		if err != nil {
			return fmt.Errorf("invalid --to date: %w", err)
		}
		patch["toDate"] = t.Format(time.RFC3339)
	}

	req := &jmap.Request{}
	req.Invoke(&vacationresponse.Set{
		Account: vacationAccountID(c),
		Update: map[jmap.ID]jmap.Patch{
			"singleton": patch,
		},
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return fmt.Errorf("vacation set failed: %w", err)
	}

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return fmt.Errorf("JMAP error: %s", r.Error())
		case *vacationresponse.SetResponse:
			if r.NotUpdated != nil {
				if setErr, ok := r.NotUpdated["singleton"]; ok {
					return fmt.Errorf("failed to set vacation: %s", formatVacationSetError(setErr))
				}
			}
			if jsonOutput {
				return output.PrintJSON(map[string]string{"action": "enabled"})
			}
			fmt.Fprintln(os.Stderr, "Vacation auto-reply enabled.")
			return nil
		}
	}

	return fmt.Errorf("unexpected response from server")
}

func runVacationDisable(cmd *cobra.Command, args []string) error {
	c, err := client.New(tokenFlag)
	if err != nil {
		return err
	}

	req := &jmap.Request{}
	req.Invoke(&vacationresponse.Set{
		Account: vacationAccountID(c),
		Update: map[jmap.ID]jmap.Patch{
			"singleton": {
				"isEnabled": false,
			},
		},
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return fmt.Errorf("vacation disable failed: %w", err)
	}

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return fmt.Errorf("JMAP error: %s", r.Error())
		case *vacationresponse.SetResponse:
			if r.NotUpdated != nil {
				if setErr, ok := r.NotUpdated["singleton"]; ok {
					return fmt.Errorf("failed to disable vacation: %s", formatVacationSetError(setErr))
				}
			}
			if jsonOutput {
				return output.PrintJSON(map[string]string{"action": "disabled"})
			}
			fmt.Fprintln(os.Stderr, "Vacation auto-reply disabled.")
			return nil
		}
	}

	return fmt.Errorf("unexpected response from server")
}

func printVacation(v *vacationresponse.VacationResponse) error {
	if jsonOutput {
		return output.PrintJSON(v)
	}

	enabled := "no"
	if v.IsEnabled {
		enabled = "yes"
	}
	fmt.Printf("Enabled: %s\n", enabled)

	if v.Subject != nil {
		fmt.Printf("Subject: %s\n", *v.Subject)
	}
	if v.FromDate != nil {
		fmt.Printf("From:    %s\n", v.FromDate.Format("2006-01-02"))
	}
	if v.ToDate != nil {
		fmt.Printf("To:      %s\n", v.ToDate.Format("2006-01-02"))
	}
	if v.TextBody != nil {
		fmt.Printf("\n%s\n", *v.TextBody)
	}

	return nil
}

func formatVacationSetError(e *jmap.SetError) string {
	if e.Description != nil {
		return fmt.Sprintf("%s: %s", e.Type, *e.Description)
	}
	return e.Type
}
