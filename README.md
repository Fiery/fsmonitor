fsmonitor
=========

A non-blocking monitoring service concurrently notifies file system changes by periodically scanning the file system.

Developed initially for the use case when the target paths to be monitored are shared/mounted volumes where all file operations happen remotely.

### API
#### Event
- `FileCreate`
- `FileRemove`
- `FileUpdate`
- `FileRename`
  
#### Notice
- `Name() string`
  - considered to be uid
- `Type() Event`
- `Time() time.Time` 
  - timestamp when created

#### Watcher
- `Watch() (chan<- chan<- Notice, <-chan error)` 
  - returns internal controlling channels between Monitor and Watcher, doing the watching on given resource
  
#### Monitor
- `New(address string, pattern []string, watcher interface{}) *Monitor`
  - creates specified Watcher and include it in returned Monitor instance
  - wathcer can be any type implements Watcher interface, or a name string refers to one of the builtin Watchers:
  	- `"path"` scans input directory using filepath.Walk
  	- `"file"` scans a virtual file system defined by a specifically formatted text file
- `Start(sleep,  event... Event)`
  - starts Watch() goroutine and loops until internal channels closes
- `Notices() <-chan Notice`
  - channel of all notices, closes when calling Close()
- `Close()`
  - safely closes all internal channels and gracefully terminates all goroutines
    
    
### Example
a simple kafka client built atop can be found in [example](example/) folder
    

### Todo
- More events support
- Integrate with system kernel API based notification library to achieve best performance in local file monitoring use case
