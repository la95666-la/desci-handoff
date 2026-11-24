package connection

import (
	"encoding/json/v2"
	"errors"
	"time"

	"github.com/gorilla/websocket"

	"joincare_server/pkg/common/log"
	"joincare_server/pkg/notice/model"
)

const writeWait = 10 * time.Second

var engine *Engine

func InitCache(stopCh chan struct{}) {
	engine = &Engine{
		StopCh: stopCh,
		Connections: Connections{
			Set: make(map[string]*connectionEntry, 256),
		},
		UserInfos: UserInfos{
			Set: make(map[int]*UserInfo, 256),
		},
	}
	go Check()
}

func getEntry(addr string) *connectionEntry {
	engine.Connections.RLock()
	entry := engine.Connections.Set[addr]
	engine.Connections.RUnlock()
	return entry
}

func GetConnection(addr string) *websocket.Conn {
	entry := getEntry(addr)
	if entry == nil {
		return nil
	}
	conn, _ := entry.conn.(*websocket.Conn)
	return conn
}

func AddConnection(addr string, conn *websocket.Conn) {
	addSocket(addr, conn)
}

func addSocket(addr string, conn socket) {
	entry := &connectionEntry{conn: conn}
	engine.Connections.Lock()
	old := engine.Connections.Set[addr]
	engine.Connections.Set[addr] = entry
	engine.Connections.Unlock()
	if old != nil {
		_ = old.conn.Close()
	}
}

func RemoveConnection(addr string) {
	removeEntry(addr, nil)
}

func RemoveConnectionIfMatch(addr string, conn *websocket.Conn) bool {
	entry := getEntry(addr)
	if entry == nil || entry.conn != conn {
		return false
	}
	return removeEntry(addr, entry)
}

func removeEntry(addr string, expected *connectionEntry) bool {
	engine.Connections.Lock()
	entry := engine.Connections.Set[addr]
	if entry == nil || expected != nil && entry != expected {
		engine.Connections.Unlock()
		return false
	}
	delete(engine.Connections.Set, addr)
	engine.Connections.Unlock()
	_ = entry.conn.Close()
	return true
}

func AddUserInfo(addr string, uid int) {
	engine.UserInfos.Lock()
	engine.UserInfos.Set[uid] = &UserInfo{
		Uid:           uid,
		Address:       addr,
		LastHeartBeat: time.Now().Unix(),
	}
	engine.UserInfos.Unlock()
}

func Expire(uid int) {
	engine.UserInfos.Lock()
	if info := engine.UserInfos.Set[uid]; info != nil {
		info.LastHeartBeat = time.Now().Unix()
	}
	engine.UserInfos.Unlock()
}

func GeUserInfoByAddr(addr string) *UserInfo {
	engine.UserInfos.RLock()
	defer engine.UserInfos.RUnlock()
	for _, info := range engine.UserInfos.Set {
		if info.Address == addr {
			copy := *info
			return &copy
		}
	}
	return nil
}

func GetUserInfoByUid(uid int) *UserInfo {
	engine.UserInfos.RLock()
	defer engine.UserInfos.RUnlock()
	info := engine.UserInfos.Set[uid]
	if info == nil {
		return nil
	}
	copy := *info
	return &copy
}

func RemoveUser(addr string) {
	engine.UserInfos.Lock()
	for id, info := range engine.UserInfos.Set {
		if info.Address == addr {
			delete(engine.UserInfos.Set, id)
			break
		}
	}
	engine.UserInfos.Unlock()
}

func Clear() {
	engine.Connections.Lock()
	entries := make([]*connectionEntry, 0, len(engine.Connections.Set))
	for _, entry := range engine.Connections.Set {
		entries = append(entries, entry)
	}
	engine.Connections.Set = make(map[string]*connectionEntry)
	engine.Connections.Unlock()

	engine.UserInfos.Lock()
	engine.UserInfos.Set = make(map[int]*UserInfo)
	engine.UserInfos.Unlock()

	for _, entry := range entries {
		_ = entry.conn.Close()
	}
}

func SendDataByUID(uid int, event string, data any) error {
	content, err := json.Marshal(data)
	if err != nil {
		return err
	}
	userInfo := GetUserInfoByUid(uid)
	if userInfo == nil {
		return errors.New("not found")
	}
	send := model.Message{
		Time:  time.Now().Unix(),
		Event: event,
		Data:  content,
		UID:   uid,
	}
	entry := getEntry(userInfo.Address)
	if entry == nil {
		return errors.New("connection not found")
	}

	entry.writeMu.Lock()
	_ = entry.conn.SetWriteDeadline(time.Now().Add(writeWait))
	err = entry.conn.WriteJSON(send)
	entry.writeMu.Unlock()
	if err != nil {
		log.GetLogger().Errorf("Send Message err: %s", err)
		if removeEntry(userInfo.Address, entry) {
			RemoveUser(userInfo.Address)
		}
		return err
	}
	return nil
}

func Check() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case _, ok := <-engine.StopCh:
			if !ok {
				return
			}
		case <-ticker.C:
			CheckHeartBeat()
		}
	}
}

func Close(uid int, addr string) {
	engine.UserInfos.Lock()
	delete(engine.UserInfos.Set, uid)
	engine.UserInfos.Unlock()
	RemoveConnection(addr)
}

func CheckHeartBeat() {
	now := time.Now().Unix()
	var expired []string
	engine.UserInfos.Lock()
	for uid, info := range engine.UserInfos.Set {
		if now-info.LastHeartBeat >= 90 {
			expired = append(expired, info.Address)
			delete(engine.UserInfos.Set, uid)
		}
	}
	engine.UserInfos.Unlock()
	for _, addr := range expired {
		RemoveConnection(addr)
	}
}
