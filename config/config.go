package config

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

func ParseConfigurationFile(path string) Configuration {
	if len(path) == 0 {
		log.Fatal("Path to the configuration file was empty")
	}

	dat, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}

	unmarshalledConf := Configuration{}
	err = json.Unmarshal(dat, &unmarshalledConf)

	if err != nil {
		log.Fatal(err)
	}

	return unmarshalledConf
}
