package config

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"os"
	"path"
	"sync"
)

var configOnce sync.Once

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

// InitConfig initializes 'Config'. It's public so that other packages' init()
// functions can call it if they need fields in config(), but this package's
// init() function calls it as well, so non-init() code should be able to read
// Config directly
func InitConfig() {
	configOnce.Do(func() {
		// Parse config and initialize Config fields
		configpath := path.Join(os.Getenv("HOME"), ".svpconfig")
		if _, err := os.Stat(configpath); os.IsNotExist(err) {
			useDefaultConfig()
		} else {
			configfile, err := os.Open(configpath)
			if err != nil {
				log.Fatalf("could not open config file at %s for reading: %s",
					configpath, err)
			}
			buf := bytes.NewBuffer(nil)
			io.Copy(buf, configfile)
			err = json.Unmarshal(buf.Bytes(), &Config)
			if err != nil {
				log.Fatalf("could not parse ${HOME}/.svpconfig: %s", err.Error())
			}
		}
	})
}

func init() {
	InitConfig()
}

func useDefaultConfig() {
	Config.ClientDirectory = path.Join(os.Getenv("HOME"), "clients")
	Config.Diff.Tool = "meld"
}
