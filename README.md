fsmonitor
=========

Concurrent monitoring service notifies file system changes by periodically scanning the directory, filtering out changed files.

Developed for the use case when the target paths to be monitored are shared/mounted volumes where the file operations happen remotely.

### API
#### Event
- `FileCreate`
- `FileRemove`
- `FileUpdate`
- `FileRename`
  
#### Notice
- `Name() string`
- `Type() Event`
- `Time() time.Time`
  
#### Monitor
- `Watch(path string, pattern []string, event... Event)`
- `Notices() <-chan Notice`
    
    
### Example
a simple kafka client built atop can be found in /example folder, work flow is as below:
- `go monitor.Watch(...)`
- `range` over `monitor.Notices()` to collect file change notices
- Sends messages over to kafka cluster under a predefined topic using `sarama.SyncProducer`.
- In the meantime, log to kafka cluster under  `topic`.process.log topic using `sarama.AsyncProducer`.
    
Inspired by sarama's [http\_sever](https://github.com/Shopify/sarama/tree/master/examples/http_server) example
    
    
### Todo
    - More events support
    - Integrate with file system notification library to achieve best performance in local file monitoring use case.fsmonitor
