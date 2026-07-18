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
	rows, e := r.db.Query(`SELECT r.id,r.name,DATE_FORMAT(r.service_date,'%Y-%m-%d'),TIME_FORMAT(r.start_time,'%H:%i'),TIME_FORMAT(r.end_time,'%H:%i'),r.code,r.status,r.current_song_id,r.current_song_section_id,COUNT(DISTINCT tm.id),r.created_by,r.created_at,GROUP_CONCAT(DISTINCT c.name ORDER BY c.name) FROM rooms r LEFT JOIN team_members tm ON tm.room_id=r.id LEFT JOIN room_channels rc ON rc.room_id=r.id LEFT JOIN channels c ON c.id=rc.channel_id GROUP BY r.id ORDER BY r.service_date DESC,r.start_time`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []domain.Room
	for rows.Next() {
		var x domain.Room
		var cs sql.NullString
		if e = rows.Scan(&x.ID, &x.Name, &x.Date, &x.StartTime, &x.EndTime, &x.Code, &x.Status, &x.CurrentSongID, &x.CurrentSongSectionID, &x.Members, &x.CreatedBy, &x.CreatedAt, &cs); e != nil {
			return nil, e
		}
		if cs.Valid {
			x.Channels = strings.Split(cs.String, ",")
		}
		x.Songs, e = r.roomSongs(x.ID)
		if e != nil {
			return nil, e
		}
		if x.CurrentSongID != nil {
			for i := range x.Songs {
				if x.Songs[i].ID == *x.CurrentSongID {
					x.CurrentSong = &x.Songs[i]
					break
				}
			}
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
	if e = saveRoomSongs(tx, x.ID, x.Songs); e != nil {
		return x, e
	}
	if e = tx.Commit(); e != nil {
		return x, e
	}
	return r.RoomByID(x.ID)
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
	if e == nil {
		_, e = tx.Exec(`DELETE FROM room_songs WHERE room_id=?`, x.ID)
	}
	if e == nil {
		e = saveRoomSongs(tx, x.ID, x.Songs)
	}
	if e == nil {
		_, e = tx.Exec(`UPDATE rooms SET current_song_id=NULL,current_song_section_id=NULL WHERE id=? AND current_song_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM room_songs WHERE room_id=? AND song_id=rooms.current_song_id)`, x.ID, x.ID)
	}
	if e != nil {
		return x, e
	}
	if e = tx.Commit(); e != nil {
		return x, e
	}
	return r.RoomByID(x.ID)
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
func saveRoomSongs(tx *sql.Tx, roomID int64, songs []domain.Song) error {
	for i, song := range songs {
		selectedKey := strings.TrimSpace(song.SelectedKey)
		if selectedKey == "" {
			selectedKey = song.DefaultKey
		}
		if _, e := tx.Exec(`INSERT INTO room_songs(room_id,song_id,selected_key,display_order) VALUES(?,?,?,?)`, roomID, song.ID, selectedKey, i+1); e != nil {
			return e
		}
	}
	return nil
}
func (r *MySQL) roomSongs(roomID int64) ([]domain.Song, error) {
	rows, e := r.db.Query(`SELECT s.id,s.title,s.artist,s.default_key,rs.selected_key,s.bpm,s.created_by FROM room_songs rs JOIN songs s ON s.id=rs.song_id WHERE rs.room_id=? ORDER BY rs.display_order,s.title`, roomID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := make([]domain.Song, 0)
	for rows.Next() {
		var x domain.Song
		if e = rows.Scan(&x.ID, &x.Title, &x.Artist, &x.DefaultKey, &x.SelectedKey, &x.BPM, &x.CreatedBy); e != nil {
			return nil, e
		}
		x.Sections, e = r.songSections(x.ID)
		if e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
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
	rows, e := r.db.Query(`SELECT id,label,category,priority,channel,icon,vibration,is_active,sort_order FROM cues ORDER BY sort_order,id`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []domain.Cue
	for rows.Next() {
		var x domain.Cue
		if e = rows.Scan(&x.ID, &x.Label, &x.Category, &x.Priority, &x.Channel, &x.Icon, &x.Vibration, &x.Active, &x.SortOrder); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *MySQL) CreateCue(x domain.Cue) (domain.Cue, error) {
	res, e := r.db.Exec(`INSERT INTO cues(label,category,priority,channel,icon,vibration,is_active,sort_order) VALUES(?,?,?,?,?,?,?,?)`, x.Label, x.Category, x.Priority, x.Channel, x.Icon, x.Vibration, x.Active, x.SortOrder)
	if e == nil {
		x.ID, _ = res.LastInsertId()
	}
	return x, e
}
func (r *MySQL) UpdateCue(x domain.Cue) (domain.Cue, error) {
	res, e := r.db.Exec(`UPDATE cues SET label=?,category=?,priority=?,channel=?,icon=?,vibration=?,is_active=?,sort_order=? WHERE id=?`, x.Label, x.Category, x.Priority, x.Channel, x.Icon, x.Vibration, x.Active, x.SortOrder, x.ID)
	return x, affected(res, e)
}
func (r *MySQL) DeleteCue(id int64) error {
	res, e := r.db.Exec(`DELETE FROM cues WHERE id=?`, id)
	return affected(res, e)
}
func (r *MySQL) ListSongs(search string) ([]domain.Song, error) {
	q := `SELECT id,title,artist,default_key,bpm,created_by FROM songs`
	args := []any{}
	if strings.TrimSpace(search) != "" {
		q += ` WHERE title LIKE ? OR artist LIKE ?`
		term := "%" + strings.TrimSpace(search) + "%"
		args = append(args, term, term)
	}
	q += ` ORDER BY title,artist`
	rows, e := r.db.Query(q, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := make([]domain.Song, 0)
	for rows.Next() {
		var x domain.Song
		if e = rows.Scan(&x.ID, &x.Title, &x.Artist, &x.DefaultKey, &x.BPM, &x.CreatedBy); e != nil {
			return nil, e
		}
		x.Sections, e = r.songSections(x.ID)
		if e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *MySQL) SongByID(id int64) (x domain.Song, e error) {
	e = r.db.QueryRow(`SELECT id,title,artist,default_key,bpm,created_by FROM songs WHERE id=?`, id).Scan(&x.ID, &x.Title, &x.Artist, &x.DefaultKey, &x.BPM, &x.CreatedBy)
	if e != nil {
		return
	}
	x.Sections, e = r.songSections(id)
	return
}
func (r *MySQL) songSections(songID int64) ([]domain.SongSection, error) {
	rows, e := r.db.Query(`SELECT id,song_id,section_label,lyrics,display_order FROM song_sections WHERE song_id=? ORDER BY display_order,id`, songID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []domain.SongSection
	for rows.Next() {
		var x domain.SongSection
		if e = rows.Scan(&x.ID, &x.SongID, &x.SectionLabel, &x.Lyrics, &x.DisplayOrder); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *MySQL) CreateSong(x domain.Song) (domain.Song, error) { return r.saveSong(x, false) }
func (r *MySQL) UpdateSong(x domain.Song) (domain.Song, error) { return r.saveSong(x, true) }
func (r *MySQL) saveSong(x domain.Song, update bool) (domain.Song, error) {
	tx, e := r.db.Begin()
	if e != nil {
		return x, e
	}
	defer tx.Rollback()
	if update {
		_, err := tx.Exec(`UPDATE songs SET title=?,artist=?,default_key=?,bpm=? WHERE id=?`, x.Title, x.Artist, x.DefaultKey, x.BPM, x.ID)
		if err != nil {
			return x, err
		}
	} else {
		res, err := tx.Exec(`INSERT INTO songs(title,artist,default_key,bpm,created_by) VALUES(?,?,?,?,?)`, x.Title, x.Artist, x.DefaultKey, x.BPM, x.CreatedBy)
		if err != nil {
			return x, err
		}
		x.ID, _ = res.LastInsertId()
	}
	retained := make([]int64, 0, len(x.Sections))
	for i := range x.Sections {
		s := &x.Sections[i]
		s.SongID = x.ID
		if s.DisplayOrder < 1 {
			s.DisplayOrder = i + 1
		}
		if update && s.ID > 0 {
			if _, err := tx.Exec(`UPDATE song_sections SET section_label=?,lyrics=?,display_order=? WHERE id=? AND song_id=?`, s.SectionLabel, s.Lyrics, s.DisplayOrder, s.ID, x.ID); err != nil {
				return x, err
			}
			retained = append(retained, s.ID)
		} else {
			res, err := tx.Exec(`INSERT INTO song_sections(song_id,section_label,lyrics,display_order) VALUES(?,?,?,?)`, x.ID, s.SectionLabel, s.Lyrics, s.DisplayOrder)
			if err != nil {
				return x, err
			}
			s.ID, _ = res.LastInsertId()
			retained = append(retained, s.ID)
		}
	}
	if update {
		query := `DELETE FROM song_sections WHERE song_id=?`
		args := []any{x.ID}
		if len(retained) > 0 {
			query += ` AND id NOT IN (` + strings.TrimRight(strings.Repeat("?,", len(retained)), ",") + `)`
			for _, sectionID := range retained {
				args = append(args, sectionID)
			}
		}
		if _, e = tx.Exec(query, args...); e != nil {
			return x, e
		}
	}
	if e = tx.Commit(); e != nil {
		return x, e
	}
	return r.SongByID(x.ID)
}
func (r *MySQL) DeleteSong(id int64) error {
	res, e := r.db.Exec(`DELETE FROM songs WHERE id=?`, id)
	return affected(res, e)
}
func (r *MySQL) SetRoomSongState(roomID int64, songID, sectionID *int64) (domain.Room, error) {
	if songID != nil {
		var found int
		if e := r.db.QueryRow(`SELECT COUNT(*) FROM room_songs WHERE room_id=? AND song_id=?`, roomID, *songID).Scan(&found); e != nil {
			return domain.Room{}, e
		}
		if found == 0 {
			return domain.Room{}, errors.New("lagu tidak ada di setlist room")
		}
	}
	if sectionID != nil {
		var sectionSong int64
		if e := r.db.QueryRow(`SELECT song_id FROM song_sections WHERE id=?`, *sectionID).Scan(&sectionSong); e != nil {
			return domain.Room{}, e
		}
		if songID == nil || sectionSong != *songID {
			return domain.Room{}, errors.New("song section tidak sesuai dengan lagu")
		}
	}
	res, e := r.db.Exec(`UPDATE rooms SET current_song_id=?,current_song_section_id=? WHERE id=?`, songID, sectionID, roomID)
	if e = affected(res, e); e != nil {
		return domain.Room{}, e
	}
	return r.RoomByID(roomID)
}
func (r *MySQL) ListActivities(room int64) ([]domain.Activity, error) {
	rows, e := r.db.Query(`SELECT id,room_id,sender,message,target,received,song_id,song_section_id,created_at FROM activity_logs WHERE room_id=? ORDER BY created_at DESC LIMIT 100`, room)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []domain.Activity
	for rows.Next() {
		var x domain.Activity
		if e = rows.Scan(&x.ID, &x.RoomID, &x.Sender, &x.Message, &x.Target, &x.Received, &x.SongID, &x.SongSectionID, &x.CreatedAt); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *MySQL) CreateActivity(x domain.Activity) (domain.Activity, error) {
	res, e := r.db.Exec(`INSERT INTO activity_logs(room_id,sender,message,target,received,song_id,song_section_id) VALUES(?,?,?,?,?,?,?)`, x.RoomID, x.Sender, x.Message, x.Target, x.Received, x.SongID, x.SongSectionID)
	if e == nil {
		x.ID, _ = res.LastInsertId()
		if x.SongID != nil {
			song, err := r.SongByID(*x.SongID)
			if err != nil {
				return x, err
			}
			x.Song = &song
			for i := range song.Sections {
				if x.SongSectionID != nil && song.Sections[i].ID == *x.SongSectionID {
					x.SongSection = &song.Sections[i]
					break
				}
			}
		}
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
