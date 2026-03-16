package main

import (
	"fmt"
	"os"

	"github.com/joshL1215/tego/internal/filter"
	"github.com/joshL1215/tego/internal/proxy"
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

			engine := filter.NewEngine(config)
			server := proxy.NewServer(port, engine)
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

	rootCmd.AddCommand(serveCmd, testCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
