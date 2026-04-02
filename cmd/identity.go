package cmd

import (
	"fmt"
	"os"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail/identity"
	"github.com/spf13/cobra"
	"github.com/vicyap/fastmail-cli/internal/client"
	"github.com/vicyap/fastmail-cli/internal/output"
)

var identityCmd = &cobra.Command{
	Use:   "identity",
	Short: "Manage sending identities",
}

var identityListCmd = &cobra.Command{
	Use:   "list",
	Short: "List sending identities",
	RunE:  runIdentityList,
}

func init() {
	rootCmd.AddCommand(identityCmd)
	identityCmd.AddCommand(identityListCmd)
}

func runIdentityList(cmd *cobra.Command, args []string) error {
	c, err := client.New(tokenFlag)
	if err != nil {
		return err
	}

	req := &jmap.Request{}
	req.Invoke(&identity.Get{
		Account: c.MailAccountID(),
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return fmt.Errorf("identity list failed: %w", err)
	}

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return fmt.Errorf("JMAP error: %s", r.Error())
		case *identity.GetResponse:
			return printIdentities(r.List)
		}
	}

	return fmt.Errorf("unexpected response from server")
}

func printIdentities(identities []*identity.Identity) error {
	if jsonOutput {
		return output.PrintJSON(identities)
	}

	tbl := output.NewTable(os.Stdout)
	tbl.Headers("ID", "NAME", "EMAIL")
	for _, ident := range identities {
		tbl.Row(string(ident.ID), ident.Name, ident.Email)
	}
	return tbl.Flush()
}
