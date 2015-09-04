fsmonitor
=========

Concurrent monitoring service notifies file system changes by periodically scanning the directory, filtering out changed files.

Developed initially for the use case when the target paths to be monitored are shared/mounted volumes where the file operations happen remotely.

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

#### Watcher (Customizable)
- `Watch() (chan<- chan<- Notice, <-chan error)` 
  - returns internal controlling channels between Monitor and Watcher, doing the watching on given resource
  
#### Monitor
- `New(address string, pattern []string, watcher interface{}) *Monitor`
  - creates specified Watcher and include it in returned Monitor instance
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
- Integrate with file system notification library to achieve best performance in local file monitoring use case.fsmonitor
