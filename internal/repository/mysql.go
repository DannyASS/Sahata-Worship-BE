package repository

import (
	"database/sql"
	"errors"
	"sahata-worship-be/internal/domain"
	"strings"
)

type MySQL struct{ db *sql.DB }

func NewMySQL(db *sql.DB) *MySQL { return &MySQL{db: db} }
func (r *MySQL) CreateUser(u domain.User) (domain.User, error) {
	res, e := r.db.Exec(`INSERT INTO users(name,email,password_hash,role,account_status,avatar) VALUES(?,?,?,?,?,?)`, u.Name, u.Email, u.PasswordHash, u.Role, u.Status, u.Avatar)
	if e != nil {
		return u, e
	}
	u.ID, _ = res.LastInsertId()
	return u, nil
}
func (r *MySQL) UserByEmail(email string) (u domain.User, e error) {
	e = r.db.QueryRow(`SELECT id,name,email,password_hash,role,account_status,avatar,created_at FROM users WHERE email=?`, email).Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.Status, &u.Avatar, &u.CreatedAt)
	return
}
func (r *MySQL) UserByID(id int64) (u domain.User, e error) {
	e = r.db.QueryRow(`SELECT id,name,email,password_hash,role,account_status,avatar,created_at FROM users WHERE id=?`, id).Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.Status, &u.Avatar, &u.CreatedAt)
	return
}
func (r *MySQL) ListUsers() ([]domain.User, error) {
	rows, e := r.db.Query(`SELECT id,name,email,role,account_status,avatar,created_at FROM users ORDER BY FIELD(account_status,'pending','active','rejected'),created_at DESC`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []domain.User
	for rows.Next() {
		var u domain.User
		if e = rows.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.Status, &u.Avatar, &u.CreatedAt); e != nil {
			return nil, e
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
func (r *MySQL) UpdateUserAccess(id int64, role, status string) (domain.User, error) {
	res, e := r.db.Exec(`UPDATE users SET role=?,account_status=? WHERE id=?`, role, status, id)
	if e = affected(res, e); e != nil {
		return domain.User{}, e
	}
	return r.UserByID(id)
}
func (r *MySQL) ListRooms() ([]domain.Room, error) {
	rows, e := r.db.Query(`SELECT r.id,r.name,DATE_FORMAT(r.service_date,'%Y-%m-%d'),TIME_FORMAT(r.start_time,'%H:%i'),TIME_FORMAT(r.end_time,'%H:%i'),r.code,r.status,COUNT(DISTINCT tm.id),r.created_by,r.created_at,GROUP_CONCAT(DISTINCT c.name ORDER BY c.name) FROM rooms r LEFT JOIN team_members tm ON tm.room_id=r.id LEFT JOIN room_channels rc ON rc.room_id=r.id LEFT JOIN channels c ON c.id=rc.channel_id GROUP BY r.id ORDER BY r.service_date DESC,r.start_time`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []domain.Room
	for rows.Next() {
		var x domain.Room
		var cs sql.NullString
		if e = rows.Scan(&x.ID, &x.Name, &x.Date, &x.StartTime, &x.EndTime, &x.Code, &x.Status, &x.Members, &x.CreatedBy, &x.CreatedAt, &cs); e != nil {
			return nil, e
		}
		if cs.Valid {
			x.Channels = strings.Split(cs.String, ",")
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *MySQL) RoomByID(id int64) (domain.Room, error) {
	xs, e := r.ListRooms()
	if e != nil {
		return domain.Room{}, e
	}
	for _, x := range xs {
		if x.ID == id {
			return x, nil
		}
	}
	return domain.Room{}, sql.ErrNoRows
}
func (r *MySQL) CreateRoom(x domain.Room) (domain.Room, error) {
	tx, e := r.db.Begin()
	if e != nil {
		return x, e
	}
	defer tx.Rollback()
	res, e := tx.Exec(`INSERT INTO rooms(name,service_date,start_time,end_time,code,status,created_by) VALUES(?,?,?,?,?,?,?)`, x.Name, x.Date, x.StartTime, x.EndTime, x.Code, x.Status, x.CreatedBy)
	if e != nil {
		return x, e
	}
	x.ID, _ = res.LastInsertId()
	if e = saveChannels(tx, x.ID, x.Channels); e != nil {
		return x, e
	}
	return x, tx.Commit()
}
func (r *MySQL) UpdateRoom(x domain.Room) (domain.Room, error) {
	tx, e := r.db.Begin()
	if e != nil {
		return x, e
	}
	defer tx.Rollback()
	_, e = tx.Exec(`UPDATE rooms SET name=?,service_date=?,start_time=?,end_time=?,code=?,status=? WHERE id=?`, x.Name, x.Date, x.StartTime, x.EndTime, x.Code, x.Status, x.ID)
	if e != nil {
		return x, e
	}
	_, e = tx.Exec(`DELETE FROM room_channels WHERE room_id=?`, x.ID)
	if e == nil {
		e = saveChannels(tx, x.ID, x.Channels)
	}
	if e != nil {
		return x, e
	}
	return x, tx.Commit()
}
func saveChannels(tx *sql.Tx, id int64, names []string) error {
	for _, n := range names {
		_, e := tx.Exec(`INSERT INTO channels(name) VALUES(?) ON DUPLICATE KEY UPDATE name=VALUES(name)`, n)
		if e != nil {
			return e
		}
		_, e = tx.Exec(`INSERT IGNORE INTO room_channels(room_id,channel_id) SELECT ?,id FROM channels WHERE name=?`, id, n)
		if e != nil {
			return e
		}
	}
	return nil
}
func (r *MySQL) DeleteRoom(id int64) error {
	res, e := r.db.Exec(`DELETE FROM rooms WHERE id=?`, id)
	return affected(res, e)
}
func (r *MySQL) RoomByCode(code string) (domain.Room, error) {
	var id int64
	e := r.db.QueryRow(`SELECT id FROM rooms WHERE UPPER(code)=UPPER(?)`, strings.TrimSpace(code)).Scan(&id)
	if e != nil {
		return domain.Room{}, e
	}
	return r.RoomByID(id)
}
func (r *MySQL) ListMembers(room int64) ([]domain.Member, error) {
	q := `SELECT id,room_id,user_id,name,role,channel,status,headset,battery,last_active FROM team_members`
	var rows *sql.Rows
	var e error
	if room > 0 {
		rows, e = r.db.Query(q+` WHERE room_id=? ORDER BY name`, room)
	} else {
		rows, e = r.db.Query(q + ` ORDER BY name`)
	}
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []domain.Member
	for rows.Next() {
		var x domain.Member
		if e = rows.Scan(&x.ID, &x.RoomID, &x.UserID, &x.Name, &x.Role, &x.Channel, &x.Status, &x.Headset, &x.Battery, &x.LastActive); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *MySQL) CreateMember(x domain.Member) (domain.Member, error) {
	res, e := r.db.Exec(`INSERT INTO team_members(room_id,name,role,channel,status,headset,battery) VALUES(?,?,?,?,?,?,?)`, x.RoomID, x.Name, x.Role, x.Channel, x.Status, x.Headset, x.Battery)
	if e == nil {
		x.ID, _ = res.LastInsertId()
	}
	return x, e
}
func (r *MySQL) UpdateMember(x domain.Member) (domain.Member, error) {
	res, e := r.db.Exec(`UPDATE team_members SET room_id=?,name=?,role=?,channel=?,status=?,headset=?,battery=?,last_active=CURRENT_TIMESTAMP WHERE id=?`, x.RoomID, x.Name, x.Role, x.Channel, x.Status, x.Headset, x.Battery, x.ID)
	return x, affected(res, e)
}
func (r *MySQL) DeleteMember(id int64) error {
	res, e := r.db.Exec(`DELETE FROM team_members WHERE id=?`, id)
	return affected(res, e)
}
func (r *MySQL) UpsertUserMember(x domain.Member) (domain.Member, error) {
	_, e := r.db.Exec(`INSERT INTO team_members(room_id,user_id,name,role,channel,status,headset,battery) VALUES(?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE name=VALUES(name),role=VALUES(role),channel=VALUES(channel),status='connected',headset=VALUES(headset),battery=VALUES(battery),last_active=CURRENT_TIMESTAMP`, x.RoomID, x.UserID, x.Name, x.Role, x.Channel, x.Status, x.Headset, x.Battery)
	if e != nil {
		return x, e
	}
	e = r.db.QueryRow(`SELECT id,room_id,user_id,name,role,channel,status,headset,battery,last_active FROM team_members WHERE room_id=? AND user_id=?`, x.RoomID, x.UserID).Scan(&x.ID, &x.RoomID, &x.UserID, &x.Name, &x.Role, &x.Channel, &x.Status, &x.Headset, &x.Battery, &x.LastActive)
	return x, e
}
func (r *MySQL) DeleteUserMember(roomID, userID int64) (domain.Member, error) {
	var x domain.Member
	e := r.db.QueryRow(`SELECT id,room_id,user_id,name,role,channel,status,headset,battery,last_active FROM team_members WHERE room_id=? AND user_id=?`, roomID, userID).Scan(&x.ID, &x.RoomID, &x.UserID, &x.Name, &x.Role, &x.Channel, &x.Status, &x.Headset, &x.Battery, &x.LastActive)
	if e != nil {
		return x, e
	}
	_, e = r.db.Exec(`DELETE FROM team_members WHERE id=?`, x.ID)
	return x, e
}
func (r *MySQL) ListCues() ([]domain.Cue, error) {
	rows, e := r.db.Query(`SELECT id,label,category,priority,channel,icon,vibration,sort_order FROM cues ORDER BY sort_order,id`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []domain.Cue
	for rows.Next() {
		var x domain.Cue
		if e = rows.Scan(&x.ID, &x.Label, &x.Category, &x.Priority, &x.Channel, &x.Icon, &x.Vibration, &x.SortOrder); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *MySQL) CreateCue(x domain.Cue) (domain.Cue, error) {
	res, e := r.db.Exec(`INSERT INTO cues(label,category,priority,channel,icon,vibration,sort_order) VALUES(?,?,?,?,?,?,?)`, x.Label, x.Category, x.Priority, x.Channel, x.Icon, x.Vibration, x.SortOrder)
	if e == nil {
		x.ID, _ = res.LastInsertId()
	}
	return x, e
}
func (r *MySQL) UpdateCue(x domain.Cue) (domain.Cue, error) {
	res, e := r.db.Exec(`UPDATE cues SET label=?,category=?,priority=?,channel=?,icon=?,vibration=?,sort_order=? WHERE id=?`, x.Label, x.Category, x.Priority, x.Channel, x.Icon, x.Vibration, x.SortOrder, x.ID)
	return x, affected(res, e)
}
func (r *MySQL) DeleteCue(id int64) error {
	res, e := r.db.Exec(`DELETE FROM cues WHERE id=?`, id)
	return affected(res, e)
}
func (r *MySQL) ListActivities(room int64) ([]domain.Activity, error) {
	rows, e := r.db.Query(`SELECT id,room_id,sender,message,target,received,created_at FROM activity_logs WHERE room_id=? ORDER BY created_at DESC LIMIT 100`, room)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []domain.Activity
	for rows.Next() {
		var x domain.Activity
		if e = rows.Scan(&x.ID, &x.RoomID, &x.Sender, &x.Message, &x.Target, &x.Received, &x.CreatedAt); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *MySQL) CreateActivity(x domain.Activity) (domain.Activity, error) {
	res, e := r.db.Exec(`INSERT INTO activity_logs(room_id,sender,message,target,received) VALUES(?,?,?,?,?)`, x.RoomID, x.Sender, x.Message, x.Target, x.Received)
	if e == nil {
		x.ID, _ = res.LastInsertId()
	}
	return x, e
}
func (r *MySQL) GetSettings(id int64) (x domain.Settings, e error) {
	e = r.db.QueryRow(`SELECT user_id,theme,language,notifications,vibration,audio_device,mic_sensitivity,cue_volume FROM user_settings WHERE user_id=?`, id).Scan(&x.UserID, &x.Theme, &x.Language, &x.Notifications, &x.Vibration, &x.AudioDevice, &x.MicSensitivity, &x.CueVolume)
	if errors.Is(e, sql.ErrNoRows) {
		return domain.Settings{UserID: id, Theme: "dark", Language: "id", Notifications: true, Vibration: true, AudioDevice: "Default headset", MicSensitivity: 70, CueVolume: 72}, nil
	}
	return
}
func (r *MySQL) SaveSettings(x domain.Settings) (domain.Settings, error) {
	_, e := r.db.Exec(`INSERT INTO user_settings(user_id,theme,language,notifications,vibration,audio_device,mic_sensitivity,cue_volume) VALUES(?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE theme=VALUES(theme),language=VALUES(language),notifications=VALUES(notifications),vibration=VALUES(vibration),audio_device=VALUES(audio_device),mic_sensitivity=VALUES(mic_sensitivity),cue_volume=VALUES(cue_volume)`, x.UserID, x.Theme, x.Language, x.Notifications, x.Vibration, x.AudioDevice, x.MicSensitivity, x.CueVolume)
	return x, e
}
func affected(res sql.Result, e error) error {
	if e != nil {
		return e
	}
	n, e := res.RowsAffected()
	if e == nil && n == 0 {
		return sql.ErrNoRows
	}
	return e
}

var _ domain.Store = (*MySQL)(nil)
var _ = errors.Is
