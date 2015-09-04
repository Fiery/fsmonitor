package fsmonitor

import (
	"time"

	"os"
	"path/filepath"
	"regexp"
)

type Watcher interface{
	/* Requires to return a nested channel of Notice and a channel of error
	 * nested Notice channel is send-only of send-only, giving Monitor fully control
	 * error channel is receive-only, Watcher keep it fully controllable
	 */

	Watch() (chan<- chan<- Notice, <-chan error)
}

type pathScanner struct{
	address string
	pattern []regexp.Regexp
	lastCheck map[string]os.FileInfo

}

/* Traverses the given directory and sub-directories and sends changes since last check */
func (s *pathScanner) Watch() (chan<- chan<- Notice, <-chan error) {

	/*
	 * nested channel to coordinate Monitor and Watcher during termination
	 * Declared as channel of send-only channel as golang doesn't do 
	 * implicit conversion for inner channel type, though a regular channel will
	 * be converted to a send-only/receive-only channel when you send it 
	 * to a channel of send-only/receive-only channel.
	 */
	ncc := make(chan chan<- Notice)
	errors := make(chan error)

	/* Will return when ncc is closed by Monitor, and every scan is guaranteed to be complete scan
	 * as filepath.Walk is blocking operation, and close of Notice channel depends on 
	 * close of error channel from this goroutine. Monitor will choose to ignore notices
	 * up to the length of notice channel

	 * nested channel explicitly used as <-chan chan<- & status channel explicitly used as chan<-
	 */
	go func(ncc <-chan chan<- Notice, errors chan<- error){
		/* close here so to release termination handling in Watch() */
		defer close(errors)

		for changed:= range ncc{
			Logger.Printf("Scanning kicked off!")
			visited := make(map[string]os.FileInfo)
			created := 0

			err := filepath.Walk(s.address, func(file string, info os.FileInfo, err error) error {
				if info.IsDir() {
					return err
				}

				matched := false || len(s.pattern) == 0
				for _, re := range s.pattern {
					if re.FindStringIndex(file) != nil {
						matched = true
						break
					}
				}
				if !matched {
					return err
				}

				if oldinfo, ok := s.lastCheck[file]; ok {
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
				} else if s.lastCheck != nil {

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
			if s.lastCheck != nil && len(s.lastCheck) > (len(visited)-created) {
				for file, info := range s.lastCheck {
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

			s.lastCheck = visited

			Logger.Printf("Scanning finalized!")

			errors <- err
		}
	/* all passed channels will be implicitly converted to desired one */
	}(ncc, errors)
	/* all returned channels will be implicitly converted to desired one */ 
	return ncc, errors
}


type fileScanner struct{
	address string
	pattern []regexp.Regexp
	lastCheck map[string]os.FileInfo

}

/* Traverses the given directory and sub-directories and sends changes since last check */
func (s *fileScanner) Watch() (ncc chan<- chan<- Notice, errors <-chan error) {
	return
}
