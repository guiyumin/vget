package cli

import (
	"fmt"

	"github.com/guiyumin/vget/internal/core/config"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create vget config file",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Run interactive wizard (loads existing config as defaults if present)
		cfg, err := config.RunInitWizard()
		if err != nil {
			return err
		}

		// Save config
		if err := config.Save(cfg); err != nil {
			return err
		}

		fmt.Printf("\nSaved %s\n", config.SavePath())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
