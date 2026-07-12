package domain

import "time"

type User struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	Role         string    `json:"role"`
	Status       string    `json:"status"`
	Avatar       *string   `json:"avatar,omitempty"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"createdAt"`
}
type Room struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Date      string    `json:"date"`
	StartTime string    `json:"startTime"`
	EndTime   string    `json:"endTime"`
	Code      string    `json:"code"`
	Status    string    `json:"status"`
	Members   int       `json:"members"`
	Channels  []string  `json:"channels"`
	CreatedBy int64     `json:"createdBy,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}
type Member struct {
	ID         int64     `json:"id"`
	RoomID     int64     `json:"roomId"`
	UserID     *int64    `json:"userId,omitempty"`
	Name       string    `json:"name"`
	Role       string    `json:"role"`
	Channel    string    `json:"channel"`
	Status     string    `json:"status"`
	Headset    bool      `json:"headset"`
	Battery    int       `json:"battery"`
	LastActive time.Time `json:"lastActive"`
}
type Cue struct {
	ID        int64   `json:"id"`
	Label     string  `json:"label"`
	Category  string  `json:"category"`
	Priority  string  `json:"priority"`
	Channel   string  `json:"channel"`
	Icon      *string `json:"icon,omitempty"`
	Vibration *string `json:"vibration,omitempty"`
	SortOrder int     `json:"sortOrder"`
}
type Activity struct {
	ID         int64     `json:"id"`
	RoomID     int64     `json:"roomId"`
	Sender     string    `json:"sender"`
	SenderRole string    `json:"senderRole,omitempty"`
	Message    string    `json:"message"`
	Target     string    `json:"target"`
	Received   bool      `json:"received"`
	CreatedAt  time.Time `json:"createdAt"`
}
type Settings struct {
	UserID         int64  `json:"userId"`
	Theme          string `json:"theme"`
	Language       string `json:"language"`
	Notifications  bool   `json:"notifications"`
	Vibration      bool   `json:"vibration"`
	AudioDevice    string `json:"audioDevice"`
	MicSensitivity int    `json:"micSensitivity"`
	CueVolume      int    `json:"cueVolume"`
}

type Store interface {
	CreateUser(User) (User, error)
	UserByEmail(string) (User, error)
	UserByID(int64) (User, error)
	ListUsers() ([]User, error)
	UpdateUserAccess(int64, string, string) (User, error)
	ListRooms() ([]Room, error)
	RoomByID(int64) (Room, error)
	RoomByCode(string) (Room, error)
	CreateRoom(Room) (Room, error)
	UpdateRoom(Room) (Room, error)
	DeleteRoom(int64) error
	ListMembers(int64) ([]Member, error)
	CreateMember(Member) (Member, error)
	UpdateMember(Member) (Member, error)
	DeleteMember(int64) error
	UpsertUserMember(Member) (Member, error)
	DeleteUserMember(int64, int64) (Member, error)
	ListCues() ([]Cue, error)
	CreateCue(Cue) (Cue, error)
	UpdateCue(Cue) (Cue, error)
	DeleteCue(int64) error
	ListActivities(int64) ([]Activity, error)
	CreateActivity(Activity) (Activity, error)
	GetSettings(int64) (Settings, error)
	SaveSettings(Settings) (Settings, error)
}
