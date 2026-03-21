package main

import (
	"fmt"
	"os"

	"github.com/joshL1215/tego/internal/filter"
	"github.com/joshL1215/tego/internal/proxy"
	"github.com/joshL1215/tego/internal/store"
	"github.com/joshL1215/tego/internal/testmode"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "tego",
		Short: "Token Eater Go — API proxy that filters noisy CLI output to save tokens",
	}

	var port int
	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the tego proxy server",
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := filter.LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load filter config: %w", err)
			}

			s, err := store.Open()
			if err != nil {
				return fmt.Errorf("failed to open dedup store: %w", err)
			}
			defer s.Close()

			engine := filter.NewEngine(config)
			server := proxy.NewServer(port, engine, s)
			return server.Start()
		},
	}
	serveCmd.Flags().IntVar(&port, "port", 8400, "Port to listen on")

	testCmd := &cobra.Command{
		Use:   "test",
		Short: "Send a sample noisy request through the proxy to see filtering in action",
		RunE: func(cmd *cobra.Command, args []string) error {
			return testmode.Run(port)
		},
	}
	testCmd.Flags().IntVar(&port, "port", 8400, "Port to listen on")

	retrieveCmd := &cobra.Command{
		Use:   "retrieve [id]",
		Short: "Retrieve a deduplicated text block by its ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := store.Open()
			if err != nil {
				return fmt.Errorf("failed to open dedup store: %w", err)
			}
			defer s.Close()

			content, err := s.Retrieve(args[0])
			if err != nil {
				return err
			}
			fmt.Print(content)
			return nil
		},
	}

	rootCmd.AddCommand(serveCmd, testCmd, retrieveCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
