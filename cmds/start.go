/**
 * Copyright (C) 2015 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *         http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package cmds

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/fabric8io/gofabric8/util"
	"github.com/spf13/cobra"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

const (
	memory  = "memory"
	cpus    = "cpus"
	console = "console"
)

// NewCmdStart starts a local cloud environment
func NewCmdStart(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Starts a local cloud development environment",
		Long:  `Starts a local cloud development environment`,

		Run: func(cmd *cobra.Command, args []string) {

			flag := cmd.Flags().Lookup(minishift)
			isOpenshift := false
			if flag != nil {
				isOpenshift = flag.Value.String() == "true"
			}
			if isOpenshift {
				kubeBinary = minishift
			}

			// check if already running
			// TODO: should we vendor the minikube and minishift status packages rather than using exec?
			out, err := exec.Command(kubeBinary, "status").Output()
			status := strings.TrimSpace(string(out))
			if err == nil && status == "Running" {
				// already running so lets
				util.Successf("%s already running\n", kubeBinary)

			} else {
				args := []string{"start"}

				// if we're running on OSX default to using xhyve
				if runtime.GOOS == "darwin" {
					args = append(args, "--vm-driver=xhyve")
				}

				// set memory flag
				memoryValue := cmd.Flags().Lookup(memory).Value.String()
				args = append(args, "--memory="+memoryValue)

				// set cpu flag
				cpusValue := cmd.Flags().Lookup(cpus).Value.String()
				args = append(args, "--cpus="+cpusValue)

				// start the local VM
				e := exec.Command(kubeBinary, args...)
				e.Stdout = os.Stdout
				e.Stderr = os.Stderr
				err = e.Run()
				if err != nil {
					util.Errorf("Unable to start %v", err)
				}
			}

			// now check that fabric8 is running, if not deploy it
			c, err := keepTryingToGetClient(f)
			if err != nil {
				util.Fatalf("Unable to connect to %s", kubeBinary)
			}

			if isOpenshift {
				// deploy fabric8
				e := exec.Command("oc", "login", "--username=admin", "--password=admin")
				e.Stdout = os.Stdout
				e.Stderr = os.Stderr
				err = e.Run()
				if err != nil {
					util.Errorf("Unable to login %v", err)
				}

			}
			// deploy fabric8 if its not already running
			ns, _, _ := f.DefaultNamespace()
			_, err = c.Services(ns).Get("fabric8")
			if err != nil {
				args := []string{"deploy", "y"}

				flag := cmd.Flags().Lookup(console)
				if flag != nil && flag.Value.String() == "true" {
					args = append(args, "--app=")
				}

				// deploy fabric8
				e := exec.Command("gofabric8", args...)
				e.Stdout = os.Stdout
				e.Stderr = os.Stderr
				err = e.Run()
				if err != nil {
					util.Errorf("Unable to start %v", err)
				}
			} else {
				openService(ns, "fabric8", c, false)
			}
		},
	}
	cmd.PersistentFlags().BoolP(minishift, "", false, "start the openshift flavour of Kubernetes")
	cmd.PersistentFlags().BoolP(console, "", false, "start only the fabric8 console")
	cmd.PersistentFlags().StringP(memory, "", "4096", "amount of RAM allocated to the VM")
	cmd.PersistentFlags().StringP(cpus, "", "1", "number of CPUs allocated to the VM")
	return cmd
}

func keepTryingToGetClient(f *cmdutil.Factory) (*client.Client, error) {
	timeout := time.After(2 * time.Minute)
	tick := time.Tick(1 * time.Second)
	// Keep trying until we're timed out or got a result or got an error
	for {
		select {
		// Got a timeout! fail with a timeout error
		case <-timeout:
			return nil, errors.New("timed out")
		// Got a tick, try and get teh client
		case <-tick:
			c, _ := getClient(f)
			// return if we have a client
			if c != nil {
				return c, nil
			}
			util.Info("Cannot connect to api server, retrying...")
			// retry
		}
	}
}

func getClient(f *cmdutil.Factory) (*client.Client, error) {
	var err error
	cfg, err := f.ClientConfig()
	if err != nil {
		return nil, err
	}
	c, err := client.New(cfg)
	if err != nil {
		return nil, err
	}
	return c, nil
}
