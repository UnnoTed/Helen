package models

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	db "github.com/TF2Stadium/Helen/database"
	"github.com/TF2Stadium/Helen/helpers"
	"github.com/jinzhu/gorm"
)

type LobbyType int
type Whitelist int
type LobbyState int

const (
	LobbyTypeSixes      LobbyType = 0
	LobbyTypeHighlander LobbyType = 1
)

var TypePlayerCount = map[LobbyType]int{
	LobbyTypeSixes:      6,
	LobbyTypeHighlander: 9,
}

const (
	LobbyStateInitializing LobbyState = 0
	LobbyStateWaiting      LobbyState = 1
	LobbyStateInProgress   LobbyState = 2
	LobbyStateEnded        LobbyState = 3
)

var stateString = map[LobbyState]string{
	LobbyStateWaiting:    "Waiting For Players",
	LobbyStateInProgress: "Lobby in Progress",
	LobbyStateEnded:      "Lobby Ended",
}

var FormatMap = map[LobbyType]string{
	LobbyTypeSixes:      "Sixes",
	LobbyTypeHighlander: "Highlander",
}

type LobbySlot struct {
	ID uint
	// Lobby    Lobby
	LobbyId uint
	// Player   Player
	PlayerId uint
	Slot     int
	Ready    bool
}

//Given Lobby IDs are unique, we'll use them for mumble channel names
type Lobby struct {
	gorm.Model
	MapName string
	State   LobbyState
	Type    LobbyType

	Slots []LobbySlot

	Server       *Server `sql:"-"` // server
	ServerInfo   ServerRecord
	ServerInfoID uint

	Whitelist Whitelist //whitelist.tf ID

	Spectators []Player `gorm:"many2many:spectators_players_lobbies"`

	BannedPlayers []Player `gorm:"many2many:banned_players_lobbies"`

	CreatedByID uint
	CreatedBy   Player
}

func NewLobby(mapName string, lobbyType LobbyType, serverInfo ServerRecord, whitelist int) *Lobby {
	lobby := &Lobby{
		Type:       lobbyType,
		State:      LobbyStateInitializing,
		MapName:    mapName,
		Server:     nil,
		Whitelist:  Whitelist(whitelist), // that's a strange line
		ServerInfo: serverInfo,
	}

	// Must specify CreatedBy manually if the lobby is created by a player

	return lobby
}

func (lobby *Lobby) GetPlayerSlot(player *Player) (int, error) {
	slotObj := &LobbySlot{}

	err := db.DB.Where("player_id = ? AND lobby_id = ?", player.ID, lobby.ID).First(slotObj).Error

	return slotObj.Slot, err
}

func (lobby *Lobby) GetPlayerIdBySlot(slot int) (uint, error) {
	slotObj := &LobbySlot{}

	err := db.DB.Where("lobby_id = ? AND slot = ?", lobby.ID, slot).First(slotObj).Error

	return uint(slotObj.PlayerId), err
}

func (lobby *Lobby) Save() error {
	var err error
	if db.DB.NewRecord(lobby) {
		err = db.DB.Create(lobby).Error
	} else {
		err = db.DB.Save(lobby).Error
	}
	return err
}

func GetLobbyById(id uint) (*Lobby, *helpers.TPError) {
	nonExistentLobby := helpers.NewTPError("Lobby not in the database", -1)

	lob := &Lobby{}
	err := db.DB.Preload("ServerInfo").First(lob, id).Error

	if err != nil {
		return nil, nonExistentLobby
	}

	return lob, nil
}

// //Add player to lobby
func (lobby *Lobby) AddPlayer(player *Player, slot int) *helpers.TPError {
	/* Possible errors while joining
	 * Slot has been filled
	 * Player has already joined a lobby
	 * anything else?
	 */

	lobbyBanError := helpers.NewTPError("The player has been banned from this lobby.", 4)
	badSlotError := helpers.NewTPError("This slot does not exist.", 3)
	filledError := helpers.NewTPError("This slot has been filled.", 2)
	alreadyInLobbyError := helpers.NewTPError("Player is already in a lobby", 1)

	if player.ID == 0 {
		return helpers.NewTPError("Player not in the database", -1)
	}

	num := 0

	// It should really be possible to do this query using relations
	if err := db.DB.Table("banned_players_lobbies").
		Where("lobby_id = ? AND player_id = ?", lobby.ID, player.ID).
		Count(&num).Error; num > 0 || err != nil {
		helpers.Logger.Debug(fmt.Sprint(err))
		return lobbyBanError
	}

	if slot >= 2*TypePlayerCount[lobby.Type] || slot < 0 {
		return badSlotError
	}

	slotFilled := false
	if _, err := lobby.GetPlayerIdBySlot(slot); err == nil {
		slotFilled = true
	}

	playerSlot := &LobbySlot{}
	err := db.DB.Where("player_id = ?", player.ID).Find(playerSlot)

	// if the player is in a different lobby, return error
	if err == nil && playerSlot.LobbyId != lobby.ID {
		return alreadyInLobbyError
	}

	// if the slot is occupied, return error
	if slotFilled {
		return filledError
	}

	// assign the player to a new slot
	// try to remove them from the old slot (in case they are switching slots)
	lobby.RemovePlayer(player)
	// try to remove them from spectators
	lobby.RemoveSpectator(player)

	newSlotObj := &LobbySlot{
		PlayerId: player.ID,
		LobbyId:  lobby.ID,
		Slot:     slot,
	}

	db.DB.Create(newSlotObj)

	lobby.updateServerAllowedPlayers()

	return nil
}

func (lobby *Lobby) RemovePlayer(player *Player) *helpers.TPError {
	err := db.DB.Where("player_id = ? AND lobby_id = ?", player.ID, lobby.ID).Delete(&LobbySlot{}).Error
	lobby.updateServerAllowedPlayers()
	if err != nil {
		return helpers.NewTPError(err.Error(), -1)
	}
	return nil
}

func (lobby *Lobby) BanPlayer(player *Player) {
	db.DB.Model(lobby).Association("BannedPlayers").Append(player)
}

func (lobby *Lobby) ReadyPlayer(player *Player) *helpers.TPError {
	slot := &LobbySlot{}
	err := db.DB.Where("lobby_id = ? AND player_id = ?", lobby.ID, player.ID).First(slot).Error
	if err != nil {
		return helpers.NewTPError("Player is not in the lobby.", 5)
	}
	slot.Ready = true
	db.DB.Save(slot)
	return nil
}

func (lobby *Lobby) UnreadyPlayer(player *Player) *helpers.TPError {
	slot := &LobbySlot{}
	err := db.DB.Where("lobby_id = ? AND player_id = ?", lobby.ID, player.ID).First(slot).Error
	if err != nil {
		return helpers.NewTPError("Player is not in the lobby.", 5)
	}

	slot.Ready = false
	db.DB.Save(slot)
	return nil
}

func (lobby *Lobby) IsPlayerReady(player *Player) (bool, *helpers.TPError) {
	slot := &LobbySlot{}
	err := db.DB.Where("lobby_id = ? AND player_id = ?", lobby.ID, player.ID).First(slot).Error
	if err != nil {
		return false, helpers.NewTPError("Player is not in the lobby.", 5)
	}
	return slot.Ready, nil
}

func (lobby *Lobby) IsEveryoneReady() bool {
	var slots []LobbySlot
	db.DB.Where("lobby_id = ?", lobby.ID).Find(&slots)

	if len(slots) != TypePlayerCount[lobby.Type]*2 {
		return false
	}

	for _, slot := range slots {
		if !slot.Ready {
			return false
		}
	}
	return true
}

func (lobby *Lobby) IsStarted() (bool, *helpers.TPError) {
	// TODO implement
	return false, nil
}

func (lobby *Lobby) AddSpectator(player *Player) *helpers.TPError {
	if _, err := lobby.GetPlayerSlot(player); err == nil {
		return lobby.RemovePlayer(player)
	}

	err := db.DB.Model(lobby).Association("Spectators").Append(player).Error
	if err != nil {
		return helpers.NewTPError(err.Error(), -1)
	}
	return nil
}

func (lobby *Lobby) RemoveSpectator(player *Player) *helpers.TPError {
	err := db.DB.Model(lobby).Association("Spectators").Delete(player).Error
	if err != nil {
		return helpers.NewTPError(err.Error(), -1)
	}
	return nil
}

func (lobby *Lobby) GetPlayerNumber() int {
	count := 0
	err := db.DB.Table("lobby_slots").Where("lobby_id = ?", lobby.ID).Count(&count).Error
	if err != nil {
		return 0
	}
	return count
}

func (lobby *Lobby) IsFull() bool {
	return lobby.GetPlayerNumber() >= 2*TypePlayerCount[lobby.Type]
}

func (lobby *Lobby) IsSlotFilled(slot int) bool {
	_, err := lobby.GetPlayerIdBySlot(slot)
	if err != nil {
		return false
	}
	return true
}

func (lobby *Lobby) TrySettingUp() *helpers.TPError {
	if _, ok := LobbyServerSettingUp[lobby.ID]; ok {
		return helpers.NewTPError("Lobby setup already in progress", -1)
	}

	if lobby.Server == nil {
		return helpers.NewTPError("Lobby doesn't have a server attached", -1)
	}

	LobbyServerSettingUp[lobby.ID] = time.Now()

	err := lobby.Server.Setup()

	delete(LobbyServerSettingUp, lobby.ID)

	if err != nil {
		lobby.Close()
		return helpers.NewTPError(err.Error(), -1)
	}

	lobby.State = LobbyStateWaiting
	db.DB.Save(lobby)

	return nil
}

func (lobby *Lobby) AfterSave() error {
	if lobby.State == LobbyStateEnded {
		return nil
	}

	var s *Server
	s, ok := LobbyServerMap[lobby.ID]
	randBytes := make([]byte, 6)
	rand.Read(randBytes)

	if !ok {
		s = NewServer()
		s.League = LeagueEtf2l // TODO actually accept this argument
		s.Map = lobby.MapName
		s.Type = lobby.Type
		s.Info = lobby.ServerInfo
		s.LobbyId = lobby.ID
		s.ServerPassword = base64.URLEncoding.EncodeToString(randBytes)

		err := s.VerifyInfo()

		if err != nil {
			return err
		}

		s.SetupObject()

		LobbyServerMap[lobby.ID] = s
	}
	if s == nil {
		helpers.Logger.Warning("Failed to attach server to lobby ", lobby.ID)
	}
	lobby.Server = s
	return nil
}

func (lobby *Lobby) Close() {
	lobby.Server.End()
	lobby.State = LobbyStateEnded
	delete(LobbyServerSettingUp, lobby.ID)
	db.DB.Save(lobby)
}

func (lobby *Lobby) AfterDelete() error {
	lobby.Server.End()
	return nil
}

func (lobby *Lobby) AfterFind() error {
	if (lobby.ServerInfo == ServerRecord{}) {
		// hasn't been preloaded. Do that here.
		db.DB.Find(&lobby.ServerInfo, lobby.ServerInfoID)
	}

	// should still finish Find if the server fails to initialize)
	lobby.AfterSave()
	return nil
}

func (lobby *Lobby) updateServerAllowedPlayers() {
	if lobby.Server == nil {
		// helpers.Logger.Warning("Trying to update allowed players but the lobby doesn't have a server attached. This is a bug. Fix it.")
		return
	}
	var steamids []string
	db.DB.Model(&LobbySlot{}).Joins("left join players on players.id = lobby_slots.player_id").
		Where("lobby_slots.lobby_id = ?", lobby.ID).Pluck("steam_id", &steamids)

	lobby.Server.SetAllowedPlayers(steamids)
}
