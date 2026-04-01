package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail/email"
	"github.com/spf13/cobra"
	"github.com/vicyap/fastmail-cli/internal/client"
)

var (
	attachmentOutput string
)

var emailAttachmentCmd = &cobra.Command{
	Use:   "attachment <email-id> [blob-id]",
	Short: "Download email attachments",
	Long:  "Download attachments from an email. If blob-id is given, download that specific attachment. Otherwise, download all attachments.",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runEmailAttachment,
}

func init() {
	emailCmd.AddCommand(emailAttachmentCmd)
	emailAttachmentCmd.Flags().StringVarP(&attachmentOutput, "output", "o", ".", "Output directory")
}

func runEmailAttachment(cmd *cobra.Command, args []string) error {
	c, err := client.New(tokenFlag)
	if err != nil {
		return err
	}

	emailID := jmap.ID(args[0])

	// Get the email to find attachments
	req := &jmap.Request{}
	req.Invoke(&email.Get{
		Account:        c.MailAccountID(),
		IDs:            []jmap.ID{emailID},
		BodyProperties: []string{"partId", "blobId", "size", "name", "type", "disposition"},
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get email: %w", err)
	}

	var attachments []*email.BodyPart
	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return fmt.Errorf("JMAP error: %s", r.Error())
		case *email.GetResponse:
			if len(r.List) == 0 {
				return fmt.Errorf("email not found: %s", emailID)
			}
			attachments = r.List[0].Attachments
		}
	}

	if len(attachments) == 0 {
		fmt.Fprintln(os.Stderr, "No attachments found.")
		return nil
	}

	// If a specific blob ID is given, download just that one
	if len(args) > 1 {
		blobID := jmap.ID(args[1])
		for _, att := range attachments {
			if att.BlobID == blobID {
				return downloadAttachment(c, att)
			}
		}
		return fmt.Errorf("attachment with blob ID %s not found", blobID)
	}

	// Download all attachments
	for _, att := range attachments {
		if err := downloadAttachment(c, att); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to download %s: %v\n", att.Name, err)
		}
	}

	return nil
}

func downloadAttachment(c *client.Client, att *email.BodyPart) error {
	name := att.Name
	if name == "" {
		name = fmt.Sprintf("attachment-%s", att.BlobID)
	}

	outPath := filepath.Join(attachmentOutput, name)

	reader, err := c.JMAP.Download(c.MailAccountID(), att.BlobID)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", name, err)
	}
	defer reader.Close()

	file, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", outPath, err)
	}
	defer file.Close()

	written, err := io.Copy(file, reader)
	if err != nil {
		return fmt.Errorf("failed to write %s: %w", outPath, err)
	}

	fmt.Fprintf(os.Stderr, "Downloaded %s (%d bytes)\n", name, written)
	return nil
}
