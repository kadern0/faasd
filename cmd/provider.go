package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"

	"github.com/containerd/containerd"
	bootstrap "github.com/openfaas/faas-provider"
	"github.com/openfaas/faas-provider/proxy"
	"github.com/openfaas/faas-provider/types"
	"github.com/openfaas/faasd/pkg/cninetwork"
	"github.com/openfaas/faasd/pkg/provider/config"
	"github.com/openfaas/faasd/pkg/provider/handlers"
	"github.com/spf13/cobra"
)

var providerCmd = &cobra.Command{
	Use:   "provider",
	Short: "Run the faasd-provider",
	RunE:  runProvider,
}

func runProvider(_ *cobra.Command, _ []string) error {

	config, providerConfig, err := config.ReadFromEnv(types.OsEnv{})
	if err != nil {
		return err
	}

	log.Printf("faasd-provider starting..\tService Timeout: %s\n", config.WriteTimeout.String())

	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	writeHostsErr := ioutil.WriteFile(path.Join(wd, "hosts"),
		[]byte(`127.0.0.1	localhost`), workingDirectoryPermission)

	if writeHostsErr != nil {
		return fmt.Errorf("cannot write hosts file: %s", writeHostsErr)
	}

	writeResolvErr := ioutil.WriteFile(path.Join(wd, "resolv.conf"),
		[]byte(`nameserver 8.8.8.8`), workingDirectoryPermission)

	if writeResolvErr != nil {
		return fmt.Errorf("cannot write resolv.conf file: %s", writeResolvErr)
	}

	cni, err := cninetwork.InitNetwork()
	if err != nil {
		return err
	}

	client, err := containerd.New(providerConfig.Sock)
	if err != nil {
		return err
	}

	defer client.Close()

	invokeResolver := handlers.NewInvokeResolver(client)

	userSecretPath := path.Join(wd, "secrets")

	bootstrapHandlers := types.FaaSHandlers{
		FunctionProxy:        proxy.NewHandlerFunc(*config, invokeResolver),
		DeleteHandler:        handlers.MakeDeleteHandler(client, cni),
		DeployHandler:        handlers.MakeDeployHandler(client, cni, userSecretPath),
		FunctionReader:       handlers.MakeReadHandler(client),
		ReplicaReader:        handlers.MakeReplicaReaderHandler(client),
		ReplicaUpdater:       handlers.MakeReplicaUpdateHandler(client, cni),
		UpdateHandler:        handlers.MakeUpdateHandler(client, cni, userSecretPath),
		HealthHandler:        func(w http.ResponseWriter, r *http.Request) {},
		InfoHandler:          handlers.MakeInfoHandler(Version, GitCommit),
		ListNamespaceHandler: listNamespaces(),
		SecretHandler:        handlers.MakeSecretHandler(client, userSecretPath),
		LogHandler: func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				defer r.Body.Close()
			}

			w.WriteHeader(http.StatusNotImplemented)
			w.Write([]byte(`Logs are not implemented for faasd`))
		},
	}

	log.Printf("Listening on TCP port: %d\n", *config.TCPPort)
	bootstrap.Serve(&bootstrapHandlers, config)

	return nil
}

func listNamespaces() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		list := []string{""}
		out, _ := json.Marshal(list)
		w.Write(out)
	}
}
