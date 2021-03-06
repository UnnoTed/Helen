package models

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"path/filepath"

	"github.com/TF2Stadium/Helen/config"
	"github.com/TF2Stadium/Helen/helpers"
)

const (
	ConfigsPath = "/configs/"
	MapsFile    = "maps.json"
)

type League string

const (
	LeagueUgc   League = "ugc"
	LeagueEtf2l League = "etf2l"
)

func (l *League) String() string {
	return string(*l)
}

// valid leagues
var Leagues = [...]League{
	LeagueUgc,
	LeagueEtf2l,
}

// pre config list
const (
	// etf2l
	Etf2lInitConfig           = "/etf2l/etf2l.cfg"
	Etf2lBaseSixesConfig      = "/etf2l/etf2l_6v6.cfg"
	Etf2lBaseHighlanderConfig = "/etf2l/etf2l_9v9.cfg"

	// ugc
	UgcBaseSixesConfig      = "/ugc/ugc_6v_base.cfg"
	UgcBaseHighlanderConfig = "/ugc/ugc_HL_base.cfg"
)

// MapsData holds the map + config list from maps.json
var MapsData map[string]map[string]map[League]string

// configs.json
type ServerConfig struct {
	Name   string    // example: HL_stopwatch
	League League    // ugc, etf2l...
	Type   LobbyType // 9v9, 6v6...
	Data   string    // config file's text
	Map    string
}

func InitServerConfigs() error {
	// maps
	helpers.Logger.Debug("[Configs.Init] Loading maps configs...")
	mapFile, mapErr := ioutil.ReadFile(config.Constants.StaticFileLocation + ConfigsPath + MapsFile)

	if mapErr == nil {
		json.Unmarshal(mapFile, &MapsData)
		helpers.Logger.Debug("[Configs.Init] Maps configs loaded!")

	} else {
		helpers.Logger.Debug("[Configs.Init] ERROR while trying to load maps configs!")
		return mapErr
	}

	return nil
}

func NewServerConfig() *ServerConfig {
	return new(ServerConfig)
}

func (c *ServerConfig) Get() (string, error) {
	if !c.IsLeagueValid() {
		return "", errors.New("[Configs.Get]: No league specified!")
	}

	if !c.IsLobbyTypeValid() {
		return "", errors.New("[Configs.Get]: The type you specified doesn't exists!")
	}

	if c.Name == "" {
		configName, configNameErr := c.GetMapConfig(c.Map)

		if configNameErr != nil {
			return "", configNameErr
		}

		helpers.Logger.Debug("[Configs.Get]: Map config choosen: " + configName)

		if configName == "" {
			return "", errors.New("[Configs.Get]: No config name or map specified!")
		} else {
			c.Name = configName
		}
	}

	// get config's name
	cfgName, nameErr := c.GetName()

	helpers.Logger.Debug("[Configs.Get]: Config that will be used: " + cfgName)

	if nameErr != nil {
		return "", nameErr
	}

	// gets the base config for each league
	// "the config that needs to run before the map type config"
	var preConfigName string
	var etf2lPreConfig []byte
	var etf2lPreErr error

	// etf2l
	if c.League == LeagueEtf2l {
		preConfigName = Etf2lInitConfig

		var etf2lPreConfigName string
		if c.Type == LobbyTypeSixes {
			etf2lPreConfigName = Etf2lBaseSixesConfig

		} else if c.Type == LobbyTypeHighlander {
			etf2lPreConfigName = Etf2lBaseHighlanderConfig
		}

		// etf2l pre configs's pre config lol
		etf2lPreConfig, etf2lPreErr = ioutil.ReadFile(filepath.Clean(config.Constants.StaticFileLocation +
			ConfigsPath + etf2lPreConfigName))

		if etf2lPreErr == nil {
			helpers.Logger.Debug("[Configs.Init] Etf2l's server pre-configs loaded!")
		} else {
			return "", etf2lPreErr
		}

		// ugc
	} else if c.League == LeagueUgc {
		if c.Type == LobbyTypeSixes {
			preConfigName = UgcBaseSixesConfig

		} else if c.Type == LobbyTypeHighlander {
			preConfigName = UgcBaseHighlanderConfig
		}
	}

	// pre config
	preConfig, preErr := ioutil.ReadFile(filepath.Clean(config.Constants.StaticFileLocation +
		ConfigsPath + preConfigName))

	if preErr == nil {
		helpers.Logger.Debug("[Configs.Init] Server pre-configs loaded!")
	} else {
		return "", preErr
	}

	// get config file's data
	cfgData, cfgErr := ioutil.ReadFile(filepath.Clean(config.Constants.StaticFileLocation +
		ConfigsPath + "/" +
		c.League.String() + "/" +
		cfgName))

	if cfgErr == nil {
		helpers.Logger.Debug("[Configs.Init] Server configs loaded!")
	} else {
		return "", cfgErr
	}

	var cfg string

	// insert etf2l pre config into server pre config
	if c.League == LeagueEtf2l {
		cfg = string(etf2lPreConfig[:]) + string(preConfig[:]) + string(cfgData[:])
	} else {
		cfg = string(preConfig[:]) + string(cfgData[:])
	}

	return cfg, nil
}

func (c *ServerConfig) GetName() (string, error) {
	if !c.IsLeagueValid() {
		return "", errors.New("[Configs.GetName]: Invalid league!")
	}

	if !c.IsLobbyTypeValid() {
		return "", errors.New("[Configs.GetName]: Invalid LobbyType!")
	}

	// game type as string
	var t string

	if c.League == LeagueEtf2l {
		t = LobbyTypeToString(c.Type)

	} else if c.League == LeagueUgc {
		switch {
		case c.Type == LobbyTypeSixes:
			t = "6v"
		case c.Type == LobbyTypeHighlander:
			t = "HL"
		}
	}

	// build config name
	// ugc -> 6v6 = ugc_6v_koth.cfg
	cfgName := c.League.String() + "_" + t + "_" + c.Name + ".cfg"

	return cfgName, nil
}

func (c *ServerConfig) GetMapConfig(mapName string) (string, error) {
	helpers.Logger.Debug("[Configs.GetMapConfig]: Getting config for map -> [" + mapName + "]")

	var mapConfig string

	if MapsData[mapName] != nil {
		mapConfig = MapsData[mapName][LobbyTypeToString(c.Type)][c.League]
	} else {
		return "", errors.New("[Configs.GetMapConfig]: No config can be found for this map in this game type and league!")
	}

	return mapConfig, nil
}

func (c *ServerConfig) IsLobbyTypeValid() bool {
	if c.Type == LobbyTypeSixes || c.Type == LobbyTypeHighlander {
		return true
	}

	return false
}

func (c *ServerConfig) IsLeagueValid() bool {
	for i := range Leagues {
		if c.League == Leagues[i] {
			return true
		}
	}

	return false
}

func LobbyTypeToString(t LobbyType) string {
	switch {
	case t == LobbyTypeSixes:
		return "6v6"
	case t == LobbyTypeHighlander:
		return "9v9"
	}

	return ""
}
