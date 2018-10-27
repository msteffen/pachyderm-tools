package config

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path"
	"sync"
)

const configPathEnvVar = "SVPCONFIG"

var (
	// configOnce ensures that 'Config' is only initialized once
	configOnce sync.Once

	// default config path is the location of svp's config file if 'SVPCONFIG'
	// isn't set
	defaultConfigPath = path.Join(os.Getenv("HOME"), ".svpconfig")
)

// Config is a struct containing all fields defined in the .svpconfig file
// (this is how configured values can be accessed)
var Config struct {
	// The top-level directory containing all clients
	ClientDirectory string `json:client_directory`

	Diff struct {
		// The user's preferred tool for diffing branches
		Tool string `json:tool`

		// Regex to let users skip certain files in svp diff
		// TODO: Skip should be settable per client (with maybe a global default?)
		// (maybe a flag override allowed too?)
		Skip string `json:skip`
	} `json:diff`
}

func configPath() string {
	path, ok := os.LookupEnv(configPathEnvVar)
	if ok {
		return path
	}
	return defaultConfigPath
}

// InitConfig initializes 'Config'. It's public so that other packages' init()
// functions can call it if they need fields in config(), but this package's
// init() function calls it as well, so non-init() code should be able to read
// Config directly
func InitConfig() {
	configOnce.Do(func() {
		p := configPath()
		// Parse config and initialize Config fields
		if _, err := os.Stat(p); os.IsNotExist(err) {
			loadDefaultConfig()
		} else {
			cfg, err := ioutil.ReadFile(p)
			if err != nil {
				log.Fatalf("could not read contents of config file at %s: %v",
					p, err)
			}
			if err = json.Unmarshal(cfg, &Config); err != nil {
				log.Fatalf("could not parse ${HOME}/.svpconfig: %s", err.Error())
			}
		}
	})
}

func init() {
	InitConfig()
}

func loadDefaultConfig() {
	Config.ClientDirectory = path.Join(os.Getenv("HOME"), "clients")
	Config.Diff.Tool = "meld"
}
