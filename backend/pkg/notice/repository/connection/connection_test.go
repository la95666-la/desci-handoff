package connection

import (
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	commonlog "joincare_server/pkg/common/log"
)

type fakeSocket struct {
	writeStarted chan struct{}
	allowWrite   chan struct{}
	writeErr     error
	closeOnce    sync.Once
	closed       chan struct{}
}

func newFakeSocket() *fakeSocket {
	return &fakeSocket{closed: make(chan struct{})}
}

func (f *fakeSocket) Close() error {
	f.closeOnce.Do(func() { close(f.closed) })
	return nil
}

func (f *fakeSocket) SetWriteDeadline(time.Time) error { return nil }

func (f *fakeSocket) WriteJSON(any) error {
	if f.writeStarted != nil {
		select {
		case f.writeStarted <- struct{}{}:
		default:
		}
	}
	if f.allowWrite != nil {
		<-f.allowWrite
	}
	return f.writeErr
}

func initTestEngine() {
	engine = &Engine{
		Connections: Connections{Set: make(map[string]*connectionEntry)},
		UserInfos:   UserInfos{Set: make(map[int]*UserInfo)},
	}
}

func TestSlowWriteDoesNotHoldGlobalConnectionLock(t *testing.T) {
	initTestEngine()
	slow := newFakeSocket()
	slow.writeStarted = make(chan struct{}, 1)
	slow.allowWrite = make(chan struct{})
	addSocket("slow", slow)
	AddUserInfo("slow", 1)

	done := make(chan error, 1)
	go func() { done <- SendDataByUID(1, "test", map[string]string{"ok": "yes"}) }()
	select {
	case <-slow.writeStarted:
	case <-time.After(time.Second):
		t.Fatal("slow write did not start")
	}

	fast := newFakeSocket()
	mutationDone := make(chan struct{})
	go func() {
		addSocket("fast", fast)
		RemoveConnection("fast")
		close(mutationDone)
	}()
	select {
	case <-mutationDone:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("connection map mutation blocked behind slow network write")
	}
	close(slow.allowWrite)
	if err := <-done; err != nil {
		t.Fatalf("slow write returned error: %v", err)
	}
}

func TestFailedStaleWriteDoesNotRemoveReplacement(t *testing.T) {
	commonlog.Init(t.TempDir(), "ERROR", 1)
	commonlog.GetLogger().SetOutput(io.Discard)
	initTestEngine()
	stale := newFakeSocket()
	stale.writeStarted = make(chan struct{}, 1)
	stale.allowWrite = make(chan struct{})
	stale.writeErr = errors.New("write failed")
	addSocket("same", stale)
	AddUserInfo("same", 7)

	done := make(chan error, 1)
	go func() { done <- SendDataByUID(7, "test", nil) }()
	<-stale.writeStarted
	replacement := newFakeSocket()
	addSocket("same", replacement)
	AddUserInfo("same", 7)
	close(stale.allowWrite)
	if err := <-done; err == nil {
		t.Fatal("expected stale write failure")
	}
	if entry := getEntry("same"); entry == nil || entry.conn != replacement {
		t.Fatal("stale write removed replacement connection")
	}
	if info := GetUserInfoByUid(7); info == nil || info.Address != "same" {
		t.Fatal("stale write removed replacement user binding")
	}
}

func TestRemoveUserMissingAddressDoesNotDeleteUIDZero(t *testing.T) {
	initTestEngine()
	AddUserInfo("zero", 0)
	RemoveUser("missing")
	if GetUserInfoByUid(0) == nil {
		t.Fatal("missing address removed uid zero")
	}
}

func TestHeartbeatClosesExpiredConnectionsOutsideUserLock(t *testing.T) {
	initTestEngine()
	expired := newFakeSocket()
	addSocket("expired", expired)
	engine.UserInfos.Set[9] = &UserInfo{Uid: 9, Address: "expired", LastHeartBeat: time.Now().Add(-2 * time.Minute).Unix()}
	CheckHeartBeat()
	if getEntry("expired") != nil || GetUserInfoByUid(9) != nil {
		t.Fatal("expired connection was not fully removed")
	}
	select {
	case <-expired.closed:
	default:
		t.Fatal("expired socket was not closed")
	}
}
