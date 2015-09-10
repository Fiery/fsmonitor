package fsmonitor

import (
	"os"
	"strings"
	"time"
	"fmt"
)

// Event defines different types of changes.
type Event uint32

const (
	FileCreate Event = 0x01 << iota
	FileUpdate
	FileRemove
	FileRename
)

// String implements fmt.Stringer.
func (e Event) String() string {
	var s []string
	for _, strmap := range []map[Event]string{eventName} {
		for ev, str := range strmap {
			if e&ev == ev {
				s = append(s, str)
			}
		}
	}
	return strings.Join(s, "|")
}

var eventName = map[Event]string{
	FileCreate: "notice.FileCreate",
	FileRemove: "notice.FileRemove",
	FileUpdate: "notice.FileUpdate",
	FileRename: "notice.FileRename",
}


// Notice abstracts basic information needed notification.
// Also include fmt.Stringer to simplify inspecting.
type Notice interface {
	// Considered to be uid
	Name() string
	// Timestamp when created
	Time() time.Time
	Type() Event
	More() interface{}
	fmt.Stringer
}


// Implements Notice, uses file name as Notice.Name
type fileSystemNotice struct {
	path      string
	event     Event
	fileinfo  os.FileInfo
	timestamp time.Time
}

func (f *fileSystemNotice) String() string{
	return fmt.Sprintf("{%v : %v}",f.path, f.event)
}

func (f *fileSystemNotice) Name() string {
	return f.path
}

func (f *fileSystemNotice) Type() Event {
	return f.event
}

func (f *fileSystemNotice) More() interface{} {
	return f.fileinfo
}

func (f *fileSystemNotice) Time() time.Time {
	return f.timestamp
}
