package net

// Client → Server messages

type HelloMessage struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Version int    `json:"version"`
}

type InputMessage struct {
	Type        string  `json:"type"`
	Seq         uint32  `json:"seq"`
	Up          bool    `json:"up"`
	Down        bool    `json:"down"`
	Left        bool    `json:"left"`
	Right       bool    `json:"right"`
	YawDelta    float32 `json:"yawDelta"`
	Shoot       bool    `json:"shoot"`
	ClientTimeMs int64  `json:"clientTimeMs"`
}

type ReadyMessage struct {
	Type string `json:"type"`
	Ready bool  `json:"ready"`
}

// Server → Client messages

type LobbyPlayer struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Ready bool   `json:"ready"`
}

type LobbyState struct {
	Players []LobbyPlayer `json:"players"`
	State   string        `json:"state"` // "waiting", "ready", "starting"
}

type WelcomeMessage struct {
	Type      string     `json:"type"`
	PlayerID  int        `json:"playerId"`
	RoomID    string     `json:"roomId"`
	Lobby     *LobbyState `json:"lobby,omitempty"`
}

type PlayerState struct {
	ID     int     `json:"id"`
	X      float32 `json:"x"`
	Y      float32 `json:"y"`
	Yaw    float32 `json:"yaw"`
	Alive  bool    `json:"alive"`
	Score  int     `json:"score"`
}

type RoundState struct {
	State     string `json:"state"` // "waiting", "playing", "ended"
	WinnerID  int    `json:"winnerId"`
	ResetInMs int    `json:"resetInMs"`
}

type Wall struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
	W float32 `json:"w"`
	H float32 `json:"h"`
}

type SnapMessage struct {
	Type    string        `json:"type"`
	Tick    uint32        `json:"tick"`
	Players []PlayerState `json:"players"`
	Round   RoundState    `json:"round"`
	Walls   []Wall        `json:"walls"`
	Lobby   *LobbyState   `json:"lobby,omitempty"`
}
