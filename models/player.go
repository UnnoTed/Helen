package models

import (
	"github.com/TF2Stadium/PlayerStatsScraper"
	"github.com/TF2Stadium/Helen/config"
	db "github.com/TF2Stadium/Helen/database"
	"github.com/TF2Stadium/Helen/helpers"
	"github.com/jinzhu/gorm"
)

type PlayerSetting struct {
	ID       uint
	Key      string
	Value    string `sql:"size:65535"`
	PlayerID uint
}

type Player struct {
	gorm.Model
	SteamId string `sql:"unique"` // Players steam ID
	Stats   PlayerStats
	StatsID uint

	// info from steam api
	Avatar     string
	Profileurl string
	GameHours  int
	Name       string // Player name

	Settings []PlayerSetting
}

func NewPlayer(steamId string) (*Player, error) {
	player := &Player{SteamId: steamId}

	if !config.Constants.SteamApiMockUp {
		player.Stats = NewPlayerStats()

		err := player.UpdatePlayerInfo()
		if err != nil {
			return &Player{}, err
		}
	} else {
		player.Stats = PlayerStats{}
	}

	return player, nil
}

func (player *Player) Save() error {
	var err error
	if db.DB.NewRecord(player) {
		err = db.DB.Create(player).Error
	} else {
		err = db.DB.Save(player).Error
	}
	return err
}

func GetPlayerBySteamId(steamid string) (*Player, *helpers.TPError) {
	var player = Player{}
	err := db.DB.Where("steam_id = ?", steamid).First(&player).Error
	if err != nil {
		return nil, helpers.NewTPError("Player is not in the database", -1)
	}
	return &player, nil
}

func GetPlayerWithStats(steamid string) (*Player, *helpers.TPError) {
	var player = Player{}
	err := db.DB.Where("steam_id = ?", steamid).Preload("Stats").First(&player).Error
	if err != nil {
		return nil, helpers.NewTPError("Player is not in the database", -1)
	}
	return &player, nil
}

func (player *Player) GetLobbyId() (uint, *helpers.TPError) {
	playerSlot := &LobbySlot{}
	err := db.DB.Joins("INNER JOIN lobbies ON lobbies.id = lobby_slots.lobby_id").
		Where("lobby_slots.player_id = ? AND lobbies.state <> ?", player.ID, LobbyStateEnded).
		Find(playerSlot).Error

	// if the player is not in any lobby, return error
	if err != nil {
		return 0, helpers.NewTPError("Player not in any lobby", 1)
	}

	return playerSlot.LobbyId, nil
}

func (player *Player) IsSpectatingId(lobbyid uint) bool {
	count := 0
	err := db.DB.Table("spectators_players_lobbies").Where("player_id = ? AND lobby_id = ?", player.ID, lobbyid).Count(&count).Error
	if err != nil {
		return false
	}
	return count != 0

}

func (player *Player) UpdatePlayerInfo() error {
	scraper.SetSteamApiKey(config.Constants.SteamDevApiKey)
	p, playErr := GetPlayerBySteamId(player.SteamId)

	// nil = player not in db
	if playErr == nil {
		player = p
	}

	playerInfo, infoErr := scraper.GetPlayerInfo(player.SteamId)
	if infoErr != nil {
		return infoErr
	}

	// profile state is 1 when the player have a steam community profile
	if playerInfo.Profilestate == 1 && playerInfo.Visibility == "public" {
		pHours, hErr := scraper.GetTF2Hours(player.SteamId)

		if hErr != nil {
			return hErr
		}

		player.GameHours = pHours
	}

	player.Profileurl = playerInfo.Profileurl
	player.Avatar = playerInfo.Avatar
	player.Name = playerInfo.Name

	return nil
}

func (player *Player) SetSetting(key string, value string) error {
	setting := PlayerSetting{}
	err := db.DB.Where("player_id = ? AND key = ?", player.ID, key).First(&setting).Error

	setting.PlayerID = player.ID
	setting.Key = key
	setting.Value = value

	err = db.DB.Save(&setting).Error

	return err
}

func (player *Player) GetSetting(key string) (PlayerSetting, error) {
	setting := PlayerSetting{}
	err := db.DB.Where("player_id = ? AND key = ?", player.ID, key).First(&setting).Error

	return setting, err
}

func (player *Player) GetSettings() ([]PlayerSetting, error) {
	var settings []PlayerSetting
	err := db.DB.Where("player_id = ?", player.ID).Find(&settings).Error

	return settings, err
}
