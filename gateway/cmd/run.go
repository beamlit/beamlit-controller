package cmd

import (
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/beamlit/beamlit-controller/gateway/api"
	"github.com/beamlit/beamlit-controller/gateway/internal/proxy"
	"github.com/spf13/cobra"
)

var (
	listenAddr string
	apiAddr    string

	runCmd = &cobra.Command{
		Use: "run",
		Run: func(cmd *cobra.Command, args []string) {
			proxy := proxy.New()
			router := api.NewRouter(cmd.Context(), apiAddr, proxy)
			log.Printf("API listening on %s", apiAddr)
			log.Printf("HTTP listening on %s", listenAddr)
			go func() {
				if err := router.Run(cmd.Context()); err != nil {
					slog.Error("API server error", "error", err)
					os.Exit(1)
				}
			}()
			if err := http.ListenAndServe(listenAddr, proxy); err != nil {
				slog.Error("HTTP server error", "error", err)
				os.Exit(1)
			}
		},
	}
)

func registerFlags() {
	runCmd.Flags().StringVar(
		&listenAddr, "listen-addr", ":8080", "listen address is the address to listen on for HTTP requests",
	)
	runCmd.Flags().StringVar(
		&apiAddr, "api-addr", ":8081", "API address is the address of the Beamlit Proxy API",
	)
}
