package util

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"

	"github.com/imdario/mergo"
	"github.com/ohsu-comp-bio/funnel/config"
)

// MergeConfigFileWithFlags is a util used by server commands that use flags to set
// Funnel config values. These commands can also take in the path to a Funnel config file.
// This function ensures that the config gets set up properly. Flag values override values in
// the provided config file.
func MergeConfigFileWithFlags(file string, flagConf config.Config) (config.Config, error) {
	// parse config file if it exists
	conf := config.DefaultConfig()
	err := config.ParseFile(file, &conf)
	if err != nil {
		return conf, err
	}

	// file vals <- cli val
	err = mergo.MergeWithOverwrite(&conf, flagConf)
	if err != nil {
		return conf, err
	}

	defaults := config.DefaultConfig()
	if conf.Server.RPCAddress() != defaults.Server.RPCAddress() {
		if conf.Server.RPCAddress() != conf.RPCClient.ServerAddress {
			conf.RPCClient.ServerAddress = conf.Server.RPCAddress()
		}
	}

	// Make sure that User/Password are defined for conf.RPCClient:
	// 1) when conf.Server.BasicAuth has credentials
	// 2) when conf.Server.OidcAuth is enabled (clients still need to provide Basic credentials)
	if conf.RPCClient.User == "" && conf.RPCClient.Password == "" {
		if len(conf.Server.BasicAuth) > 0 {
			fmt.Println("Configuration problem: RPCClient User and Password " +
				"are undefined while Server.BasicAuth is enforced.")
			os.Exit(1)
		} else if conf.Server.OidcAuth.ServiceConfigURL != "" {
			// Generating random user/password credentials for RPC:
			conf.RPCClient.User = randomCredential()
			conf.RPCClient.Password = randomCredential()
			conf.Server.BasicAuth = append(conf.Server.BasicAuth, config.BasicCredential{
				User:     conf.RPCClient.User,
				Password: conf.RPCClient.Password,
			})
		}
	}
	return conf, nil
}

// TempConfigFile writes the configuration to a temporary file.
// Returns:
// - "path" is the path of the file.
// - "cleanup" can be called to remove the temporary file.
func TempConfigFile(c config.Config, name string) (path string, cleanup func()) {
	tmpdir, err := os.MkdirTemp("", "")
	if err != nil {
		panic(err)
	}

	cleanup = func() {
		os.RemoveAll(tmpdir)
	}

	p := filepath.Join(tmpdir, name)
	err = config.ToYamlFile(c, p)
	if err != nil {
		panic(err)
	}
	return p, cleanup
}

// Generates a random string of length 20.
func randomCredential() string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	b := make([]rune, 20)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}
