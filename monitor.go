package fsmonitor

import (
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"os"
	"path/filepath"
	"regexp"
)

const (
	notice_buffer_length = 1000
)

type Monitor struct {
	notices chan Notice
	closing chan chan error

	lastCheckpoint map[string]os.FileInfo
}

var Logger = log.New(ioutil.Discard, "[Monitor] ", log.LstdFlags)

func (m *Monitor) Watch(address string, pattern []string, sleep time.Duration, event ...Event){
	Logger.Printf("Starting monitoring file system changes on %s", address)
	
	/* pattern filtering, return fatal status when pattern doesn't compile correctly. */
	var patexp = make([]regexp.Regexp, 0)
	for _, pat := range pattern {
		if exp, err := regexp.Compile(pat); err != nil {

			Logger.Fatalln("Pattern string failed compilation, please check syntax!", err)
		} else {
			patexp = append(patexp, *exp)

		}
	}

	var n Notice
	var tc chan error
	var ec = make(chan error)
	/* Use nested channel to coordinate Watch() and scan() during termination
	 * Declared as channel of send-only channel as golang doesn't do 
	 * implicit conversion for inner channel type, though a regular channel will
	 * be converted to a send-only channel when you send it to a channel of send-only channel.
	 */
	var ncc =make(chan chan<- Notice)
	var nbuf = make(chan Notice, notice_buffer_length)
	var time_tick = time.Tick(sleep)
	/* Kick off check goroutin here and use for range loop to avoid contention
	 * by blocking only one scan() goroutine for the Notice channel
	 * 
	 * Why not `go scan()` every tick:
	 * every scan() goroutines may be running even slower than ticking rate,
	 * in which case there will be unbounded number of scan() goroutines generated and all blocking 
	 * on fetching Notice channel.
	 * And it surely comes the contention at the time when the oldest scan() goroutine returns to error channel.
	 * 
	 */
	go m.scan(address, patexp, ncc, ec)

	for {
		select {
		case tc = <-m.closing:
			Logger.Println("Returning from scanning loop...")
			/* close so scan() can return */
			close(ncc)
		case <-time_tick:
			time_tick = nil
			ncc<-nbuf
		case n = <-nbuf:
			for _, e := range event {
				if e == n.Type() {
					Logger.Printf("File change noticed: %v: %s", n.Type(), n.Name())
					m.notices<-n
				}
			}
		/* use error channel to indicate accomplishment of scan() */
		case err , ok:= <-ec:
			if !ok{
				/* scan() closes status channel, which means it returns due to close of channel of notice channel */
				select{
				case n=<-nbuf:
					Logger.Printf("System interrupt! %d buffered notices ignored from: %v: %s", len(nbuf)+1, n.Type(), n.Name())
				default:
					Logger.Printf("System interrupt! no buffered notices ignored.")
				}
				/* notice channel can safely close as scan() has returned already */
				close(nbuf)
				/* returns nil error as no error is supposed to show up in this block */
				tc <- nil
				return

			} else if err != nil {
				Logger.Printf("Error occured while scanning, break for a while and continue: %v", err)
				time_tick = time.After(time.Now().Add(100 * time.Second).Sub(time.Now()))
			} else {
				time_tick = time.Tick(sleep)
			}
		}
	}
}

func (m *Monitor) Notices()  (<-chan Notice){
	return m.notices
}
func (m *Monitor) Close() error {
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

func New() *Monitor {
	return &Monitor{
		notices:  make(chan Notice),
		closing:  make(chan chan error),
	}
}


/* Function actaully doing the traversing job
 * nested channel explicitly used as <-chan chan<- & status channel explicitly used as chan<- (zen of go)
 */
func (m *Monitor) scan(address string, pattern []regexp.Regexp, ncc <-chan chan<- Notice, errors chan<- error) {
	/* close here so to release termination handling in Watch() */
	defer close(errors)
	/* Will return when ncc is closed by Watch(), and every scan is guaranteed to be complete scan
	 * as filepath.Walk is blocking operation, and close of Notice channel depends on 
	 * close of error channel from this goroutine. Watch() will choose to ignore notices
	 * up to the length of notice channel
	 */
	for changed:= range ncc{
		Logger.Printf("Scanning kicked off!")
		visited := make(map[string]os.FileInfo)
		created := 0

		err := filepath.Walk(address, func(file string, info os.FileInfo, err error) error {
			if info.IsDir() {
				return err
			}

			matched := false || len(pattern) == 0
			for _, re := range pattern {
				if re.FindStringIndex(file) != nil {
					matched = true
					break
				}
			}
			if !matched {
				return err
			}

			if oldinfo, ok := m.lastCheckpoint[file]; ok {
				if info.ModTime().After(oldinfo.ModTime()) {
					changed <- &fileSystemNotice{
						path:      file,
						fileinfo:  info,
						timestamp: time.Now(),
						event:     FileUpdate,
					}
				} else if oldinfo.Size() != info.Size() {
					changed <- &fileSystemNotice{
						path:      file,
						fileinfo:  info,
						timestamp: time.Now(),
						event:     FileUpdate,
					}
				}
			} else if m.lastCheckpoint != nil {

				changed <- &fileSystemNotice{
					path:      file,
					fileinfo:  info,
					timestamp: time.Now(),
					event:     FileCreate,
				}
				created += 1
			}
			visited[file] = info

			return err
		})
		if m.lastCheckpoint != nil && len(m.lastCheckpoint) > (len(visited)-created) {
			for file, info := range m.lastCheckpoint {
				if _, ok := visited[file]; !ok {
					changed <- &fileSystemNotice{
						path:      file,
						fileinfo:  info,
						timestamp: time.Now(),
						event:     FileRemove,
					}
				}
			}
		}

		m.lastCheckpoint = visited

		Logger.Printf("Scanning finalized!")

		errors <- err
	}
}

