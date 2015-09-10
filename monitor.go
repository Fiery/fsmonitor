// Package fsmonitor is a non-blocking monitoring library concurrently notifies file system changes by periodically scanning the file system. 
// Changed notices can be filtered by event type (create, update, remove, etc.) or by resource names.
package fsmonitor

import (
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"regexp"
)

const (
	notice_buffer_length = 1000
)

// Monitor initializes environment, coordinates with Watchers and collects events.
type Monitor struct {
	notices chan Notice
	closing chan chan error

	watcher Watcher	
}

var Logger = log.New(ioutil.Discard, "[Monitor] ", log.LstdFlags)


// Start starts Wathcer goroutine and loops until internal channels closes.
func (m *Monitor) Start(sleep time.Duration, event ...Event){

	var returning chan error

	var noticeBuffer = make(chan Notice, notice_buffer_length)
	var timeTick = time.Tick(sleep)

	/* Kick off watcher goroutine here and use for range loop to avoid contention
	 * by blocking only one scan() goroutine for the Notice channel
	 * 
	 * Why not start new goroutine every tick:
	 * check goroutine may be running even slower than ticking rate,
	 * in which case there will be unbounded number of scan() goroutines generated and all blocking 
	 * on fetching Notice channel.
	 * And it surely comes the contention at the time when the oldest scan() goroutine returns to error channel.
	 * 
	 */

	ncc, errorCheck := m.watcher.Watch()

	for {
		select {
		case returning = <-m.closing:
			Logger.Println("Returning from scanning loop...")
			/* close so scan() can return */
			close(ncc)
		case <-timeTick:
			timeTick = nil
			ncc<-noticeBuffer
		case n := <-noticeBuffer:
			for _, e := range event {
				if e == n.Type() {
					Logger.Printf("File change noticed: %v", n)
					m.notices<-n
				}
			}
		/* use error channel to indicate accomplishment of every check from Watcher */
		// still selectable after closing errorCheck, even without ok check
		case err , ok:= <-errorCheck:
			if !ok{
				/* scan() closes status channel, which means it returns due to close of channel of notice channel */
				select{
				/* check buffered notice */
				case n:=<-noticeBuffer:
					Logger.Printf("System interrupt! %d buffered notices ignored from %v", len(noticeBuffer)+1, n)
				default:
					Logger.Printf("System interrupt! no buffered notice ignored.")
				}
				/* notice channel can safely close as scan() has returned already */
				close(noticeBuffer)
				/* returns nil error as no error is supposed to show up in this block */
				returning <- nil
				return

			} else if err != nil {
				Logger.Printf("Error occured while scanning, break for a while and continue: %v", err)
				timeTick = time.After(time.Now().Add(100 * time.Second).Sub(time.Now()))
			} else {
				timeTick = time.Tick(sleep)
			}
		}
	}
}


// Notices returns channel of all notices, which to be closed when calling Close().
func (m *Monitor) Notices()  (<-chan Notice){
	return m.notices
}

// Stop safely closes all internal channels and gracefully terminates all goroutines.
func (m *Monitor) Stop() error {
	var err error
	stopper := make(chan error)

	/* terminate scan loop */
	m.closing <- stopper

	/* Block until Watch() for select loop return */
	if e := <-stopper; e != nil {
		err = fmt.Errorf("%vScanner Error: %v\n", err, e)
		Logger.Fatalln("Failed to stop scanner gracefully!", e)
	}
	close(m.notices)

	Logger.Printf("Event channel successfully closed!\n")

	return err
}

// New creates specified Watcher and include it in returned Monitor instance.
func New(address string, pattern []string, watcher interface{}) *Monitor {

	/* pattern filtering, return fatal status when pattern doesn't compile correctly. */
	var patexp = make([]regexp.Regexp, len(pattern), len(pattern))
	for _, pat := range pattern {
		if exp, err := regexp.Compile(pat); err != nil {

			Logger.Fatalln("Pattern string failed compilation, please check syntax!", err)
		} else {
			patexp = append(patexp, *exp)

		}
	}

	switch tw:= watcher.(type){
	default:
		Logger.Fatalln("Watcher type not recognized! %T",tw)
	case string:
		switch tw{
		case "path":
		return &Monitor{
			notices: make(chan Notice),
			closing: make(chan chan error),
			watcher: &pathScanner{
				address: address,
				pattern: patexp,
			},
		}
		case "file":
		return &Monitor{
			notices: make(chan Notice),
			closing: make(chan chan error),
			watcher: &fileScanner{
				address: address,
				pattern: patexp,
			},
		}
		default:
			/* must provide valid watcher type */
			Logger.Fatalln("Watcher name not recognized!")
		}
	case Watcher:
		return &Monitor{
			notices: make(chan Notice),
			closing: make(chan chan error),
			watcher: tw,
		}

	}
	return nil
}


