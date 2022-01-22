package kafka

import (
	"context"
	"github.com/Shopify/sarama"
	"github.com/eolinker/eosc"
	"github.com/eolinker/eosc/formatter"
	"github.com/eolinker/eosc/log"
	"sync"
)

type Output struct {
	*Driver
	id        string
	wg        *sync.WaitGroup
	input     chan *sarama.ProducerMessage
	producer  sarama.AsyncProducer
	conf      *ProducerConfig
	cancel    context.CancelFunc
	context   context.Context
	enable    bool
	locker    *sync.Mutex
	formatter eosc.IFormatter
}

func (o *Output) Output(entry eosc.IEntry) error {
	if o.formatter != nil {
		data := o.formatter.Format(entry)
		msg := &sarama.ProducerMessage{
			Topic: o.conf.Topic,
			Value: sarama.ByteEncoder(data),
		}
		if o.conf.PartitionType == "manual" {
			msg.Partition = o.conf.Partition
		}
		if o.conf.PartitionType == "hash" {
			msg.Key = sarama.StringEncoder(entry.Read(o.conf.PartitionKey))
		}
		o.write(msg)
	}
	return nil
}

func (o *Output) Id() string {
	return o.id
}

func (o *Output) Start() error {
	return nil
}

func (o *Output) Reset(conf interface{}, workers map[eosc.RequireId]interface{}) error {
	cfg, err := o.Driver.check(conf)
	if err != nil {
		return err
	}
	// 新建formatter
	factory, has := formatter.GetFormatterFactory(cfg.Type)
	if !has {
		return errorFormatterType
	}
	o.formatter, err = factory.Create(cfg.Formatter)

	if o.producer != nil {
		// 确保关闭
		o.close()
	}
	// 新建生产者
	o.producer, err = sarama.NewAsyncProducer(cfg.Address, cfg.Conf)
	if err != nil {
		return err
	}
	o.conf = cfg
	go o.work()
	return nil
}

func (o *Output) Stop() error {
	o.close()
	o.formatter = nil
	return nil
}

func (o *Output) CheckSkill(skill string) bool {
	return false
}

func (o *Output) write(msg *sarama.ProducerMessage) {
	// 未开启情况下不给写
	if !o.enable {
		return
	}
	o.locker.Lock()
	o.input <- msg
	o.locker.Unlock()
}

func (o *Output) work() {
	if o.enable {
		return
	}
	ctx, cancelFunc := context.WithCancel(context.Background())
	o.context = ctx
	o.cancel = cancelFunc
	// 初始化消息通道
	if o.input == nil {
		o.input = make(chan *sarama.ProducerMessage)
	}
	if o.wg == nil {
		o.wg = &sync.WaitGroup{}
	}
	o.enable = true
	o.wg.Add(1)
	for {
		select {
		case <-o.context.Done():
			// 读完
			for e := range o.producer.Errors() {
				log.Warnf("kafka error:%s", e.Error())
			}
			o.wg.Done()
			return
		case msg := <-o.input:
			o.producer.Input() <- msg
		case err := <-o.producer.Errors():
			log.Warnf("kafka error:%s", err.Error())
		}
	}
}

func (o *Output) close() {
	isClose := false
	o.producer.AsyncClose()
	if o.cancel != nil {
		isClose = true
		o.cancel()
		o.cancel = nil
	}
	if isClose {
		// 等待消息都读完
		o.wg.Done()
	}
	o.producer = nil
	o.enable = false
}
