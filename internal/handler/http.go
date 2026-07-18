package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"sahata-worship-be/internal/domain"
	"sahata-worship-be/internal/usecase"
	"strconv"
	"strings"
	"sync"
)

type HTTP struct {
	u           *usecase.Service
	origin      string
	mu          sync.RWMutex
	roomStreams map[int64]map[chan streamEvent]struct{}
	directors   map[int64]map[string]map[string]string
	speakers    map[int64]string
}

type streamEvent struct {
	Name string
	Data any
}
type signalMessage struct {
	ClientID string          `json:"clientId"`
	TargetID string          `json:"targetId"`
	Type     string          `json:"type"`
	Data     json.RawMessage `json:"data"`
}

func New(u *usecase.Service, origin string) http.Handler {
	h := &HTTP{u: u, origin: origin, roomStreams: make(map[int64]map[chan streamEvent]struct{}), directors: make(map[int64]map[string]map[string]string), speakers: make(map[int64]string)}
	m := http.NewServeMux()
	m.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) { write(w, 200, map[string]string{"status": "ok"}) })
	m.HandleFunc("POST /api/v1/auth/register", h.register)
	m.HandleFunc("POST /api/v1/auth/login", h.login)
	m.HandleFunc("POST /api/v1/join", h.join)
	m.HandleFunc("GET /api/v1/rooms/{id}/events", h.roomEvents)
	m.HandleFunc("POST /api/v1/rooms/{id}/signals", h.roomSignal)
	m.HandleFunc("DELETE /api/v1/rooms/{id}/members/{memberId}", h.leaveRoom)
	m.HandleFunc("GET /api/v1/rooms/{id}/directors", h.listDirectors)
	m.Handle("POST /api/v1/rooms/{id}/presence", h.auth(http.HandlerFunc(h.userPresence)))
	m.Handle("DELETE /api/v1/rooms/{id}/presence", h.auth(http.HandlerFunc(h.userPresence)))
	m.Handle("POST /api/v1/rooms/{id}/director-presence", h.auth(http.HandlerFunc(h.directorPresence)))
	m.Handle("DELETE /api/v1/rooms/{id}/director-presence", h.auth(http.HandlerFunc(h.directorPresence)))
	m.Handle("POST /api/v1/rooms/{id}/speaker-lock", h.auth(http.HandlerFunc(h.speakerLock)))
	m.Handle("/api/v1/", h.auth(http.HandlerFunc(h.api)))
	return h.cors(m)
}
func (h *HTTP) listDirectors(w http.ResponseWriter, r *http.Request) {
	roomID, e := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if e != nil {
		write(w, 400, map[string]string{"error": "room tidak valid"})
		return
	}
	h.mu.RLock()
	list := make([]map[string]string, 0, len(h.directors[roomID]))
	for _, director := range h.directors[roomID] {
		list = append(list, director)
	}
	h.mu.RUnlock()
	write(w, 200, map[string]any{"data": list})
}

func (h *HTTP) directorPresence(w http.ResponseWriter, r *http.Request) {
	roomID, e := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if e != nil || roomID < 1 {
		write(w, 400, map[string]string{"error": "room tidak valid"})
		return
	}
	user, e := h.u.Store().UserByID(userID(r))
	if e != nil {
		fail(w, e)
		return
	}
	if user.Role != "Music Director" && user.Role != "Admin Gereja" {
		write(w, 403, map[string]string{"error": "akses khusus director"})
		return
	}
	clientID := "director-" + strconv.FormatInt(user.ID, 10)
	h.mu.Lock()
	if h.directors[roomID] == nil {
		h.directors[roomID] = make(map[string]map[string]string)
	}
	if r.Method == "POST" {
		h.directors[roomID][clientID] = map[string]string{"clientId": clientID, "name": user.Name, "role": user.Role}
	} else {
		delete(h.directors[roomID], clientID)
		if h.speakers[roomID] == clientID {
			delete(h.speakers, roomID)
		}
	}
	list := make([]map[string]string, 0, len(h.directors[roomID]))
	for _, director := range h.directors[roomID] {
		list = append(list, director)
	}
	h.mu.Unlock()
	action := "joined"
	if r.Method == "DELETE" {
		action = "left"
	}
	h.broadcast(roomID, streamEvent{Name: "director", Data: map[string]any{"action": action, "clientId": clientID, "directors": list}})
	write(w, 200, map[string]any{"data": map[string]any{"clientId": clientID, "directors": list}})
}

func (h *HTTP) speakerLock(w http.ResponseWriter, r *http.Request) {
	roomID, e := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if e != nil || roomID < 1 {
		write(w, 400, map[string]string{"error": "room tidak valid"})
		return
	}
	user, e := h.u.Store().UserByID(userID(r))
	if e != nil {
		fail(w, e)
		return
	}
	if user.Role != "Music Director" && user.Role != "Admin Gereja" {
		write(w, 403, map[string]string{"error": "akses khusus director"})
		return
	}
	var input struct {
		Action string `json:"action"`
	}
	if decode(w, r, &input) != nil {
		return
	}
	clientID := "director-" + strconv.FormatInt(user.ID, 10)
	h.mu.Lock()
	current := h.speakers[roomID]
	granted := false
	if input.Action == "acquire" && (current == "" || current == clientID) {
		h.speakers[roomID] = clientID
		current = clientID
		granted = true
	}
	if input.Action == "release" && current == clientID {
		delete(h.speakers, roomID)
		current = ""
		granted = true
	}
	h.mu.Unlock()
	if !granted {
		write(w, 409, map[string]string{"error": "director lain sedang berbicara"})
		return
	}
	h.broadcast(roomID, streamEvent{Name: "speaker", Data: map[string]string{"clientId": current, "name": user.Name}})
	write(w, 200, map[string]any{"data": map[string]any{"granted": true, "clientId": current}})
}

func (h *HTTP) roomEvents(w http.ResponseWriter, r *http.Request) {
	roomID, e := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if e != nil || roomID < 1 {
		write(w, 400, map[string]string{"error": "room tidak valid"})
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		write(w, 500, map[string]string{"error": "stream tidak didukung"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	stream := make(chan streamEvent, 32)
	h.mu.Lock()
	if h.roomStreams[roomID] == nil {
		h.roomStreams[roomID] = make(map[chan streamEvent]struct{})
	}
	h.roomStreams[roomID][stream] = struct{}{}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.roomStreams[roomID], stream)
		if len(h.roomStreams[roomID]) == 0 {
			delete(h.roomStreams, roomID)
		}
		h.mu.Unlock()
	}()
	w.Write([]byte(": connected\n\n"))
	if room, roomErr := h.u.Store().RoomByID(roomID); roomErr == nil {
		payload, _ := json.Marshal(room)
		_, _ = w.Write([]byte("event: room-state\ndata: " + string(payload) + "\n\n"))
	}
	flusher.Flush()
	for {
		select {
		case event := <-stream:
			payload, _ := json.Marshal(event.Data)
			_, _ = w.Write([]byte("event: " + event.Name + "\ndata: " + string(payload) + "\n\n"))
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (h *HTTP) broadcastActivity(activity domain.Activity) {
	h.broadcast(activity.RoomID, streamEvent{Name: "cue", Data: activity})
}

func (h *HTTP) broadcast(roomID int64, event streamEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for stream := range h.roomStreams[roomID] {
		select {
		case stream <- event:
		default:
		}
	}
}

func (h *HTTP) roomSignal(w http.ResponseWriter, r *http.Request) {
	roomID, e := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if e != nil || roomID < 1 {
		write(w, 400, map[string]string{"error": "room tidak valid"})
		return
	}
	var signal signalMessage
	if decode(w, r, &signal) != nil {
		return
	}
	if signal.ClientID == "" || signal.Type == "" || len(signal.Data) == 0 {
		fail(w, usecase.ErrInvalid)
		return
	}
	h.broadcast(roomID, streamEvent{Name: "signal", Data: signal})
	write(w, http.StatusAccepted, map[string]any{"data": map[string]string{"status": "relayed"}})
}

func (h *HTTP) leaveRoom(w http.ResponseWriter, r *http.Request) {
	roomID, roomErr := strconv.ParseInt(r.PathValue("id"), 10, 64)
	memberID, memberErr := strconv.ParseInt(r.PathValue("memberId"), 10, 64)
	if roomErr != nil || memberErr != nil || roomID < 1 || memberID < 1 {
		write(w, 400, map[string]string{"error": "room atau member tidak valid"})
		return
	}
	members, e := h.u.Store().ListMembers(roomID)
	if e != nil {
		fail(w, e)
		return
	}
	found := false
	for _, member := range members {
		if member.ID == memberID {
			found = true
			break
		}
	}
	if !found {
		write(w, 404, map[string]string{"error": "member tidak ditemukan pada room"})
		return
	}
	if e = h.u.Store().DeleteMember(memberID); e != nil {
		fail(w, e)
		return
	}
	h.broadcast(roomID, streamEvent{Name: "presence", Data: map[string]any{"action": "left", "memberId": memberID}})
	write(w, http.StatusOK, map[string]any{"data": map[string]string{"status": "left"}})
}
func (h *HTTP) userPresence(w http.ResponseWriter, r *http.Request) {
	roomID, e := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if e != nil || roomID < 1 {
		write(w, 400, map[string]string{"error": "room tidak valid"})
		return
	}
	user, e := h.u.Store().UserByID(userID(r))
	if e != nil {
		fail(w, e)
		return
	}
	if user.Role != "Member" {
		write(w, 403, map[string]string{"error": "presence ini khusus role Member"})
		return
	}
	if r.Method == "POST" {
		var input struct {
			Channel string `json:"channel"`
			Headset bool   `json:"headset"`
		}
		if decode(w, r, &input) != nil {
			return
		}
		if input.Channel == "" {
			input.Channel = "All Team"
		}
		member, createErr := h.u.Store().UpsertUserMember(domain.Member{RoomID: roomID, UserID: &user.ID, Name: user.Name, Role: user.Role, Channel: input.Channel, Status: "connected", Headset: input.Headset, Battery: 100})
		if createErr != nil {
			fail(w, createErr)
			return
		}
		h.broadcast(roomID, streamEvent{Name: "presence", Data: map[string]any{"action": "joined", "member": member}})
		write(w, http.StatusOK, map[string]any{"data": member})
		return
	}
	member, deleteErr := h.u.Store().DeleteUserMember(roomID, user.ID)
	if deleteErr != nil {
		fail(w, deleteErr)
		return
	}
	h.broadcast(roomID, streamEvent{Name: "presence", Data: map[string]any{"action": "left", "memberId": member.ID}})
	write(w, http.StatusOK, map[string]any{"data": map[string]string{"status": "left"}})
}
func (h *HTTP) join(w http.ResponseWriter, r *http.Request) {
	var x struct {
		Code    string `json:"code"`
		Name    string `json:"name"`
		Role    string `json:"role"`
		Channel string `json:"channel"`
		Headset bool   `json:"headset"`
	}
	if decode(w, r, &x) != nil {
		return
	}
	if strings.TrimSpace(x.Code) == "" || strings.TrimSpace(x.Name) == "" || strings.TrimSpace(x.Role) == "" {
		fail(w, usecase.ErrInvalid)
		return
	}
	if x.Role == "Music Director" || x.Role == "Admin Gereja" {
		write(w, http.StatusForbidden, map[string]string{"error": "role tersebut tidak tersedia untuk guest"})
		return
	}
	room, e := h.u.Store().RoomByCode(x.Code)
	if e != nil {
		fail(w, e)
		return
	}
	if x.Channel == "" {
		x.Channel = "All Team"
	}
	member, e := h.u.Store().CreateMember(domain.Member{RoomID: room.ID, Name: strings.TrimSpace(x.Name), Role: x.Role, Channel: x.Channel, Status: "connected", Headset: x.Headset, Battery: 100})
	if e != nil {
		fail(w, e)
		return
	}
	room.Members++
	h.broadcast(room.ID, streamEvent{Name: "presence", Data: map[string]any{"action": "joined", "member": member}})
	write(w, http.StatusCreated, map[string]any{"data": map[string]any{"room": room, "member": member}})
}
func (h *HTTP) api(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/"), "/"), "/")
	if r.Header.Get("X-User-Role") == "Member" && !memberAccessAllowed(parts[0], r.Method) {
		write(w, http.StatusForbidden, map[string]string{"error": "role Member tidak memiliki akses untuk aksi ini"})
		return
	}
	id := int64(0)
	if len(parts) > 1 {
		id, _ = strconv.ParseInt(parts[1], 10, 64)
	}
	s := h.u.Store()
	var v any
	var e error
	status := 200
	if parts[0] == "rooms" && len(parts) > 2 && parts[2] == "song-state" {
		if r.Method != "PUT" {
			notAllowed(w)
			return
		}
		var x struct {
			SongID        *int64 `json:"songId"`
			SongSectionID *int64 `json:"songSectionId"`
		}
		if decode(w, r, &x) != nil {
			return
		}
		v, e = s.SetRoomSongState(id, x.SongID, x.SongSectionID)
		if e == nil {
			h.broadcast(id, streamEvent{Name: "room-state", Data: v})
		}
		if e != nil {
			fail(w, e)
			return
		}
		write(w, status, map[string]any{"data": v})
		return
	}
	switch parts[0] {
	case "users":
		admin, adminErr := s.UserByID(userID(r))
		if adminErr != nil || admin.Role != "Admin Gereja" || admin.Status != "active" {
			write(w, http.StatusForbidden, map[string]string{"error": "akses khusus Admin Gereja"})
			return
		}
		switch r.Method {
		case "GET":
			v, e = s.ListUsers()
		case "PUT":
			if id == userID(r) {
				write(w, 422, map[string]string{"error": "admin tidak dapat mengubah akses akun sendiri"})
				return
			}
			var x struct {
				Role   string `json:"role"`
				Status string `json:"status"`
			}
			if decode(w, r, &x) != nil {
				return
			}
			if !validUserRole(x.Role) || !validAccountStatus(x.Status) {
				fail(w, usecase.ErrInvalid)
				return
			}
			v, e = s.UpdateUserAccess(id, x.Role, x.Status)
		default:
			notAllowed(w)
			return
		}
	case "rooms":
		switch r.Method {
		case "GET":
			if id > 0 {
				v, e = s.RoomByID(id)
			} else {
				v, e = s.ListRooms()
			}
		case "POST":
			var x domain.Room
			if decode(w, r, &x) != nil {
				return
			}
			x.CreatedBy = userID(r)
			if e = usecase.ValidateRoom(x); e == nil {
				v, e = s.CreateRoom(x)
				status = 201
			}
		case "PUT":
			var x domain.Room
			if decode(w, r, &x) != nil {
				return
			}
			x.ID = id
			if e = usecase.ValidateRoom(x); e == nil {
				v, e = s.UpdateRoom(x)
			}
		case "DELETE":
			e = s.DeleteRoom(id)
			v = map[string]string{"message": "room deleted"}
		default:
			notAllowed(w)
			return
		}
	case "members":
		switch r.Method {
		case "GET":
			room, _ := strconv.ParseInt(r.URL.Query().Get("roomId"), 10, 64)
			v, e = s.ListMembers(room)
		case "POST":
			var x domain.Member
			if decode(w, r, &x) != nil {
				return
			}
			v, e = s.CreateMember(x)
			status = 201
		case "PUT":
			var x domain.Member
			if decode(w, r, &x) != nil {
				return
			}
			x.ID = id
			v, e = s.UpdateMember(x)
		case "DELETE":
			e = s.DeleteMember(id)
			v = map[string]string{"message": "member deleted"}
		default:
			notAllowed(w)
			return
		}
	case "cues":
		switch r.Method {
		case "GET":
			v, e = s.ListCues()
		case "POST":
			var x domain.Cue
			if decode(w, r, &x) != nil {
				return
			}
			v, e = s.CreateCue(x)
			status = 201
		case "PUT":
			var x domain.Cue
			if decode(w, r, &x) != nil {
				return
			}
			x.ID = id
			v, e = s.UpdateCue(x)
		case "DELETE":
			e = s.DeleteCue(id)
			v = map[string]string{"message": "cue deleted"}
		default:
			notAllowed(w)
			return
		}
	case "songs":
		switch r.Method {
		case "GET":
			if id > 0 {
				v, e = s.SongByID(id)
			} else {
				v, e = s.ListSongs(r.URL.Query().Get("search"))
			}
		case "POST":
			var x domain.Song
			if decode(w, r, &x) != nil {
				return
			}
			x.CreatedBy = userID(r)
			if strings.TrimSpace(x.Title) == "" || strings.TrimSpace(x.Artist) == "" || strings.TrimSpace(x.DefaultKey) == "" || x.BPM < 1 || len(x.Sections) < 1 {
				fail(w, usecase.ErrInvalid)
				return
			}
			v, e = s.CreateSong(x)
			status = 201
		case "PUT":
			var x domain.Song
			if decode(w, r, &x) != nil {
				return
			}
			x.ID = id
			if strings.TrimSpace(x.Title) == "" || strings.TrimSpace(x.Artist) == "" || strings.TrimSpace(x.DefaultKey) == "" || x.BPM < 1 || len(x.Sections) < 1 {
				fail(w, usecase.ErrInvalid)
				return
			}
			v, e = s.UpdateSong(x)
		case "DELETE":
			e = s.DeleteSong(id)
			v = map[string]string{"message": "song deleted"}
		default:
			notAllowed(w)
			return
		}
	case "activities":
		switch r.Method {
		case "GET":
			room, _ := strconv.ParseInt(r.URL.Query().Get("roomId"), 10, 64)
			v, e = s.ListActivities(room)
		case "POST":
			var x domain.Activity
			if decode(w, r, &x) != nil {
				return
			}
			v, e = s.CreateActivity(x)
			if e == nil {
				h.broadcastActivity(v.(domain.Activity))
			}
			status = 201
		default:
			notAllowed(w)
			return
		}
	case "settings":
		switch r.Method {
		case "GET":
			v, e = s.GetSettings(userID(r))
		case "PUT":
			var x domain.Settings
			if decode(w, r, &x) != nil {
				return
			}
			x.UserID = userID(r)
			v, e = s.SaveSettings(x)
		default:
			notAllowed(w)
			return
		}
	default:
		http.NotFound(w, r)
		return
	}
	if e != nil {
		fail(w, e)
		return
	}
	write(w, status, map[string]any{"data": v})
}
func (h *HTTP) register(w http.ResponseWriter, r *http.Request) {
	var x struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if decode(w, r, &x) != nil {
		return
	}
	u, _, e := h.u.Register(domain.User{Name: x.Name, Email: x.Email, Role: "Member"}, x.Password)
	if e != nil {
		fail(w, e)
		return
	}
	write(w, 201, map[string]any{"data": map[string]any{"user": u, "message": "Registrasi berhasil dan menunggu approval Admin Gereja"}})
}
func (h *HTTP) login(w http.ResponseWriter, r *http.Request) {
	var x struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if decode(w, r, &x) != nil {
		return
	}
	u, t, e := h.u.Login(x.Email, x.Password)
	if e != nil {
		if errors.Is(e, usecase.ErrPending) || errors.Is(e, usecase.ErrRejected) {
			write(w, http.StatusForbidden, map[string]string{"error": e.Error()})
			return
		}
		write(w, 401, map[string]string{"error": "email atau password salah"})
		return
	}
	write(w, 200, map[string]any{"data": map[string]any{"user": u, "token": t}})
}
func validUserRole(role string) bool {
	switch role {
	case "Admin Gereja", "Music Director", "Member":
		return true
	}
	return false
}
func validAccountStatus(status string) bool {
	return status == "pending" || status == "active" || status == "rejected"
}
func (h *HTTP) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		id, e := h.u.ParseToken(v)
		if e != nil {
			write(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		user, e := h.u.Store().UserByID(id)
		if e != nil || user.Status != "active" {
			write(w, 401, map[string]string{"error": "akun belum aktif atau akses telah dinonaktifkan"})
			return
		}
		r.Header.Set("X-User-ID", strconv.FormatInt(id, 10))
		r.Header.Set("X-User-Role", user.Role)
		next.ServeHTTP(w, r)
	})
}
func memberAccessAllowed(resource, method string) bool {
	if resource == "settings" {
		return method == "GET" || method == "PUT"
	}
	switch resource {
	case "rooms", "members", "cues", "activities", "songs":
		return method == "GET"
	}
	return false
}
func (h *HTTP) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestOrigin := r.Header.Get("Origin")
		for _, allowed := range strings.Split(h.origin, ",") {
			if strings.TrimSpace(allowed) == requestOrigin {
				w.Header().Set("Access-Control-Allow-Origin", requestOrigin)
				w.Header().Set("Vary", "Origin")
				break
			}
		}
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func userID(r *http.Request) int64 {
	x, _ := strconv.ParseInt(r.Header.Get("X-User-ID"), 10, 64)
	return x
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		write(w, 400, map[string]string{"error": "JSON tidak valid: " + e.Error()})
		return e
	}
	return nil
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, e error) {
	status := 500
	if errors.Is(e, usecase.ErrInvalid) {
		status = 422
	} else if usecase.IsNotFound(e) {
		status = 404
	}
	msg := "internal server error"
	if status < 500 {
		msg = e.Error()
	}
	write(w, status, map[string]string{"error": msg})
}
func notAllowed(w http.ResponseWriter) {
	write(w, 405, map[string]string{"error": "method not allowed"})
}
