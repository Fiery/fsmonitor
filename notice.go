package fsmonitor

import (
	"os"
	"strings"
	"time"
)

type Event uint32

const (
	FileCreate Event = 0x01 << iota
	FileUpdate
	FileRemove
	FileRename
)

/* String implements fmt.Stringer interface. */
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

type Notice interface {
	Name() string
	Time() time.Time
	Type() Event
	More() interface{}
}

type fileSystemNotice struct {
	path      string
	event     Event
	fileinfo  os.FileInfo
	timestamp time.Time
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
