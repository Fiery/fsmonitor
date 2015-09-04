fsmonitor example
=================

Monitors file system change and send messages over to the Kafka cluster


### Workflow
- `monitor = fsmonitor.New(...) && go monitor.Start(...)`
- `range` over `monitor.Notices()` to collect file change notices
- Sends messages over to kafka cluster under a predefined `topic` using `sarama.SyncProducer`.
- In the meantime, log to kafka cluster under  `topic`.process.log topic using `sarama.AsyncProducer`.

Inspired by sarama's [http\_sever](https://github.com/Shopify/sarama/tree/master/examples/http_server) example