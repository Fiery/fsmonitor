package main 

import (

	"crypto/tls"
	"crypto/x509"
	"encoding/json"

	"fmt"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"
	"monitor"

	"github.com/Shopify/sarama"
)

var (
	keyFile   = flag.String("key", "", "The optional key file for client authentication")
	certFile  = flag.String("certificate", "", "The optional certificate file for client authentication")
	authFile  = flag.String("ca", "", "The optional certificate authority file for TLS client authentication")
	verifySSL = flag.Bool("verify", false, "Optional verify ssl certificates chain")

	verbose   = flag.Bool("verbose", false, "Turn on Sarama logging")
	brokers   = flag.String("brokers", os.Getenv("KAFKA_PEERS"), "The Kafka brokers to connect to, as a comma separated list")
	address   = flag.String("address", ".", "File system path to be monitored")
	pattern   = flag.String("pattern", "", "File patterns of interest")

	timeout  = flag.Int("timeout", 10000, "Timeout for file operations")
	sleep = flag.Int("sleep", 10, "Checking period in second")
	topic = flag.String("topic","monitor", "Kafka topics to be stored")

)

var Logger = log.New(os.Stdout, "[Main] ", log.LstdFlags)
var noticeSender sarama.SyncProducer
var	noticeLogger sarama.AsyncProducer

func main() {

	flag.Parse()
	if *verbose {
		sarama.Logger = log.New(os.Stdout, "[Sarama] ", log.LstdFlags)

		monitor.Logger = log.New(os.Stdout, "[Monitor] ", log.LstdFlags)
	}


	if *brokers == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}
	tlsConfig:=createTLSConfiguration()

	noticeSender = *newSyncProducer(tlsConfig, strings.Split(*brokers,","))

	noticeLogger = *newAsyncProducer(tlsConfig, strings.Split(*brokers,","))

	m:=monitor.New()

	/* Handles Ctrl+C signal */
	go func() {
		sc := make(chan os.Signal, 1)
		signal.Notify(sc, os.Interrupt)
		select{
		case <-sc:
			if err := m.Close(); err != nil {
				Logger.Fatalln("Error closing the monitor", err)
			}
		}
	}()

	go m.Watch(*address, strings.Split(*pattern,","), time.Duration(*sleep)*time.Second, monitor.FileCreate)

	/* Notice channel will be guaranteed closed after m.Close returns */
	for n:= range m.Notices(){
		logging(forwarding()).Process(n)
	}


	if e := noticeSender.Close(); e != nil {
		Logger.Fatalln("Failed to shut down event noticeSender gently!", e)
	}

	if e := noticeLogger.Close(); e != nil {
		Logger.Fatalln("Failed to shut down event noticeLogger gently!", e)
	}
	//panic("show me the stack")
}



func createTLSConfiguration() (t *tls.Config) {
	if *certFile != "" && *keyFile != "" && *authFile != "" {
		cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
		if err != nil {
			log.Fatal(err)
		}

		caCert, err := ioutil.ReadFile(*authFile)
		if err != nil {
			log.Fatal(err)
		}

		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		t = &tls.Config{
			Certificates:       []tls.Certificate{cert},
			RootCAs:            caCertPool,
			InsecureSkipVerify: *verifySSL,
		}
	}
	// will be nil by default if nothing is provided
	return t
}




func forwarding(pl ...ProcessLogic) ProcessLogic {
	return eventProcessor(func(n monitor.Notice) {

		for _, p := range pl {
			p.Process(n)
		}

		/* If no message key set, all messages will be distributed randomly
		 * over the different partitions.*/
		partition, offset, err := noticeSender.SendMessage(&sarama.ProducerMessage{
			Topic: *topic,
			Key:   sarama.StringEncoder(fmt.Sprintf("%v", n.Type())),
			Value: sarama.StringEncoder(n.Name()),
		})

		if err != nil {
			Logger.Printf("Failed to store your data:, %s", err)
		} else {
			// The tuple (topic, partition, offset) can be used as a unique identifier

			// for a message in a Kafka cluster.
			Logger.Printf("Your data is stored with unique identifier kafka://%s/%d/%d", *topic, partition, offset)
		}
	})
}

func logging(pl ...ProcessLogic) ProcessLogic {

	return eventProcessor(func(n monitor.Notice) {
		//started := time.Now()

		for _, p := range pl {
			p.Process(n)
		}

		entry := &processLogEntry{
			Path:  n.Name(),
			Event: fmt.Sprintf("%v", n.Type()),
		}

		// We will use the client's IP address as key. This will cause
		// all the access log entries of the same IP address to end up
		// on the same partition.
		noticeLogger.Input() <- &sarama.ProducerMessage{
			Topic: fmt.Sprintf("%s.process.log", *topic),
			Key:   sarama.StringEncoder(fmt.Sprintf("%s", time.Now())),
			Value: entry,
		}
	})
}

type ProcessLogic interface {
	Process(monitor.Notice)
}

type eventProcessor func(monitor.Notice)

/* Implements monitor.ProcessLogic */
func (ep eventProcessor) Process(n monitor.Notice) {
	ep(n)
}

/* Implements sarama.Encoder */
type processLogEntry struct {
	Path  string `json:"path"`
	Event string `json:"event"`

	/* saram.Encoder reserved fields */
	encoded []byte
	err     error
}

func (ple *processLogEntry) ensureEncoded() {
	if ple.encoded == nil && ple.err == nil {
		ple.encoded, ple.err = json.Marshal(ple)
	}
}

func (ple *processLogEntry) Length() int {
	ple.ensureEncoded()
	return len(ple.encoded)
}

func (ple *processLogEntry) Encode() ([]byte, error) {
	ple.ensureEncoded()
	return ple.encoded, ple.err
}

func newSyncProducer(tlsConfig *tls.Config, brokerList []string) *sarama.SyncProducer {
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll // Wait for all in-sync replicas to ack the message
	config.Producer.Retry.Max = 10                   // Retry up to 10 times to produce the message

	/* On the broker side, you may want to change the following settings to get
	 * stronger consistency guarantees:
	 * - For your broker, set `unclean.leader.election.enable` to false
	 * - For the topic, you could increase `min.insync.replicas`.*/

	producer, err := sarama.NewSyncProducer(brokerList, config)
	if err != nil {
		log.Fatalln("Failed to start Sarama producer:", err)
	}

	return &producer

}

func newAsyncProducer(tlsConfig *tls.Config, brokerList []string) *sarama.AsyncProducer {
	config := sarama.NewConfig()

	config.Producer.RequiredAcks = sarama.WaitForLocal       // Only wait for the leader to ack
	config.Producer.Compression = sarama.CompressionSnappy   // Compress messages
	config.Producer.Flush.Frequency = 500 * time.Millisecond // Flush batches every 500ms

	// On the broker side, you may want to change the following settings to get
	// stronger consistency guarantees:
	// - For your broker, set `unclean.leader.election.enable` to false
	// - For the topic, you could increase `min.insync.replicas`.

	producer, err := sarama.NewAsyncProducer(brokerList, config)
	if err != nil {
		log.Fatalln("Failed to start Sarama producer:", err)
	}

	// We will just log to STDOUT if we're not able to produce messages.
	// Note: messages will only be returned here after all retry attempts are exhausted.
	// this goroutine will eventually exit as producer.shutdown closes the errors channel
	go func() {
		for err := range producer.Errors() {
			log.Println("Failed to write access log entry:", err)
		}
	}()

	return &producer

}


