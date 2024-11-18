package cmd

import (
	"log"
	"net/http"

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
				router.Run(cmd.Context())
			}()
			if err := http.ListenAndServe(listenAddr, proxy); err != nil {
				log.Fatalf("HTTP server error: %v", err)
			}
		},
	}
)

func registerFlags() {
	runCmd.Flags().StringVar(&listenAddr, "listen-addr", ":8080", "listen address is the address to listen on for HTTP requests")
	runCmd.Flags().StringVar(&apiAddr, "api-addr", ":8081", "API address is the address of the Beamlit Proxy API")
}
