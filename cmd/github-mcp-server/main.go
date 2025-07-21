package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/github/github-mcp-server/internal/ghmcp"
	"github.com/github/github-mcp-server/pkg/github"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// These variables are set by the build process using ldflags.
var version = "version"
var commit = "commit"
var date = "date"

var (
	rootCmd = &cobra.Command{
		Use:     "github-mcp-server",
		Short:   "GitHub MCP Server",
		Long:    `A GitHub MCP server that handles various tools and resources with authentication support.`,
		Version: fmt.Sprintf("Version: %s\nCommit: %s\nBuild Date: %s", version, commit, date),
	}

	stdioCmd = &cobra.Command{
		Use:   "stdio",
		Short: "Start stdio server",
		Long:  `Start a server that communicates via standard input/output streams using JSON-RPC messages.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			token := viper.GetString("personal_access_token")
			if token == "" {
				return errors.New("GITHUB_PERSONAL_ACCESS_TOKEN not set")
			}

			// If you're wondering why we're not using viper.GetStringSlice("toolsets"),
			// it's because viper doesn't handle comma-separated values correctly for env
			// vars when using GetStringSlice.
			// https://github.com/spf13/viper/issues/380
			var enabledToolsets []string
			if err := viper.UnmarshalKey("toolsets", &enabledToolsets); err != nil {
				return fmt.Errorf("failed to unmarshal toolsets: %w", err)
			}

			stdioServerConfig := ghmcp.StdioServerConfig{
				Version:              version,
				Host:                 viper.GetString("host"),
				Token:                token,
				EnabledToolsets:      enabledToolsets,
				DynamicToolsets:      viper.GetBool("dynamic_toolsets"),
				ReadOnly:             viper.GetBool("read-only"),
				ExportTranslations:   viper.GetBool("export-translations"),
				EnableCommandLogging: viper.GetBool("enable-command-logging"),
				LogFilePath:          viper.GetString("log-file"),
			}

			return ghmcp.RunStdioServer(stdioServerConfig)
		},
	}

	sseCmd = &cobra.Command{
		Use:   "sse",
		Short: "Start SSE server with optional authentication support",
		Long:  `Start a server that communicates via Server-Sent Events over HTTP with optional authentication from gateway headers.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			token := viper.GetString("personal_access_token")
			if token == "" {
				// Check if authentication will be required
				allowUnauthenticated := viper.GetBool("allow_unauthenticated")
				if !allowUnauthenticated {
					fmt.Fprintf(os.Stderr, "Warning: No GITHUB_PERSONAL_ACCESS_TOKEN set and authentication required. Server will rely on gateway headers.\n")
				}
			}

			var enabledToolsets []string
			if err := viper.UnmarshalKey("toolsets", &enabledToolsets); err != nil {
				return fmt.Errorf("failed to unmarshal toolsets: %w", err)
			}

			port := os.Getenv("PORT")
			if port == "" {
				port = "8080"
			}

			// Use the existing SSEServerConfig structure
			sseServerConfig := ghmcp.SSEServerConfig{
				Version:              version,
				Host:                 viper.GetString("host"),
				Token:                token,
				EnabledToolsets:      enabledToolsets,
				DynamicToolsets:      viper.GetBool("dynamic_toolsets"),
				ReadOnly:             viper.GetBool("read-only"),
				ExportTranslations:   viper.GetBool("export-translations"),
				EnableCommandLogging: viper.GetBool("enable-command-logging"),
				LogFilePath:          viper.GetString("log-file"),
				ListenAddr:           ":" + port,
				BaseURL:              viper.GetString("base-url"),
				BasePath:             "",
				KeepAlive:            true,
				KeepAliveInterval:    30 * time.Second,
			}

			// Use the new authentication-aware SSE server instead of the original
			allowUnauthenticated := viper.GetBool("allow_unauthenticated")
			return ghmcp.RunSSEServerWithSimpleAuth(sseServerConfig, allowUnauthenticated)
		},
	}
)

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.SetVersionTemplate("{{.Short}}\n{{.Version}}\n")

	// Add global flags that will be shared by all commands
	rootCmd.PersistentFlags().StringSlice("toolsets", github.DefaultTools, "An optional comma separated list of groups of tools to allow, defaults to enabling all")
	rootCmd.PersistentFlags().Bool("dynamic-toolsets", false, "Enable dynamic toolsets")
	rootCmd.PersistentFlags().Bool("read-only", false, "Restrict the server to read-only operations")
	rootCmd.PersistentFlags().String("log-file", "", "Path to log file")
	rootCmd.PersistentFlags().Bool("enable-command-logging", false, "When enabled, the server will log all command requests and responses to the log file")
	rootCmd.PersistentFlags().Bool("export-translations", false, "Save translations to a JSON file")
	rootCmd.PersistentFlags().String("gh-host", "", "Specify the GitHub hostname (for GitHub Enterprise etc.)")

	// Bind flag to viper
	_ = viper.BindPFlag("toolsets", rootCmd.PersistentFlags().Lookup("toolsets"))
	_ = viper.BindPFlag("dynamic_toolsets", rootCmd.PersistentFlags().Lookup("dynamic-toolsets"))
	_ = viper.BindPFlag("read-only", rootCmd.PersistentFlags().Lookup("read-only"))
	_ = viper.BindPFlag("log-file", rootCmd.PersistentFlags().Lookup("log-file"))
	_ = viper.BindPFlag("enable-command-logging", rootCmd.PersistentFlags().Lookup("enable-command-logging"))
	_ = viper.BindPFlag("export-translations", rootCmd.PersistentFlags().Lookup("export-translations"))
	_ = viper.BindPFlag("host", rootCmd.PersistentFlags().Lookup("gh-host"))

	// Add SSE-specific flags
	sseCmd.Flags().String("base-url", "", "Base URL for the SSE server")
	sseCmd.Flags().Bool("allow-unauthenticated", false, "Allow unauthenticated requests (for testing)")

	_ = viper.BindPFlag("base-url", sseCmd.Flags().Lookup("base-url"))
	_ = viper.BindPFlag("allow_unauthenticated", sseCmd.Flags().Lookup("allow-unauthenticated"))

	// Add subcommands
	rootCmd.AddCommand(stdioCmd)
	rootCmd.AddCommand(sseCmd)
}

func initConfig() {
	// Initialize Viper configuration
	viper.SetEnvPrefix("github")
	viper.AutomaticEnv()
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
