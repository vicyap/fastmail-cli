package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestBuildCommandSchema_Simple(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "A test command",
		Long:  "A longer description.",
	}

	schema := buildCommandSchema(cmd, false)
	assert.Equal(t, "test", schema.Name)
	assert.Equal(t, "A test command", schema.Short)
	assert.Equal(t, "A longer description.", schema.Long)
	assert.Empty(t, schema.Subcommands)
}

func TestBuildCommandSchema_WithFlags(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test",
	}
	cmd.Flags().StringP("name", "n", "default", "A name flag")
	cmd.Flags().BoolP("verbose", "v", false, "Be verbose")

	schema := buildCommandSchema(cmd, false)
	assert.Len(t, schema.Flags, 2)

	// Find the name flag
	var nameFlag *flagSchema
	for i, f := range schema.Flags {
		if f.Name == "name" {
			nameFlag = &schema.Flags[i]
		}
	}
	assert.NotNil(t, nameFlag)
	assert.Equal(t, "n", nameFlag.Shorthand)
	assert.Equal(t, "default", nameFlag.Default)
	assert.Equal(t, "A name flag", nameFlag.Usage)
}

func TestBuildCommandSchema_WithSubcommands(t *testing.T) {
	parent := &cobra.Command{
		Use:   "parent",
		Short: "Parent command",
	}
	child1 := &cobra.Command{
		Use:   "child1",
		Short: "First child",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	child2 := &cobra.Command{
		Use:   "child2",
		Short: "Second child",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	parent.AddCommand(child1, child2)

	schema := buildCommandSchema(parent, true)
	assert.Len(t, schema.Subcommands, 2)
	assert.Equal(t, "child1", schema.Subcommands[0].Name)
	assert.Equal(t, "child2", schema.Subcommands[1].Name)
}

func TestBuildCommandSchema_NoRecurse(t *testing.T) {
	parent := &cobra.Command{
		Use:   "parent",
		Short: "Parent command",
	}
	child := &cobra.Command{
		Use:   "child",
		Short: "Child command",
	}
	parent.AddCommand(child)

	schema := buildCommandSchema(parent, false)
	assert.Empty(t, schema.Subcommands)
}

func TestBuildCommandSchema_HiddenFlags(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test",
	}
	cmd.Flags().String("visible", "", "Visible flag")
	cmd.Flags().String("hidden", "", "Hidden flag")
	cmd.Flags().MarkHidden("hidden")

	schema := buildCommandSchema(cmd, false)
	assert.Len(t, schema.Flags, 1)
	assert.Equal(t, "visible", schema.Flags[0].Name)
}

func TestRootCmd(t *testing.T) {
	cmd := RootCmd()
	assert.NotNil(t, cmd)
	assert.Equal(t, "fm", cmd.Name())
}
