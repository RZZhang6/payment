package main

import (
	"github.com/RZZhang6/common"
	"github.com/RZZhang6/payment/domain/repository"
	service2 "github.com/RZZhang6/payment/domain/service"
	"github.com/RZZhang6/payment/handler"
	payment "github.com/RZZhang6/payment/proto/payment"
	"github.com/jinzhu/gorm"
	"github.com/micro/go-micro/v2"
	log "github.com/micro/go-micro/v2/logger"
	"github.com/micro/go-micro/v2/registry"
	"github.com/micro/go-plugins/registry/consul/v2"
	"github.com/micro/go-plugins/wrapper/monitoring/prometheus/v2"
	ratelimit "github.com/micro/go-plugins/wrapper/ratelimiter/uber/v2"
	opentracing2 "github.com/micro/go-plugins/wrapper/trace/opentracing/v2"
	"github.com/opentracing/opentracing-go"

	_ "github.com/jinzhu/gorm/dialects/mysql"
)

func main() {
	// 配置中心
	consulConfig, err := common.GetConsulConfig("192.168.153.135", 8500, "/micro/config")
	if err != nil {
		common.Error(err)
	}
	//注册中心
	consul := consul.NewRegistry(func(options *registry.Options) {
		options.Addrs = []string{
			"192.168.153.135:8500",
		}
	})
	//jaeger 链路追踪
	tracer, io, err := common.NewTracer("go.micro.service.payment", "192.168.153.135:6831")
	if err != nil {
		common.Error(err)
	}
	defer io.Close()
	opentracing.SetGlobalTracer(tracer)

	//mysql 设置
	mysqlInfo := common.GetMysqlFromConsul(consulConfig, "mysql")
	//初始化数据库
	db, err := gorm.Open("mysql", mysqlInfo.User+":"+mysqlInfo.Pwd+"@/"+mysqlInfo.Database+"?charset=utf8&parseTime=True&loc=Local")
	if err != nil {
		common.Error(err)
	}
	defer db.Close()
	//禁止复数表
	db.SingularTable(true)

	//创建表
	/*tableInit := repository.NewPaymentRepository(db)
	tableInit.InitTable()*/

	// 监控
	common.PrometheusBoot(9089)

	// New Service
	service := micro.NewService(
		micro.Name("go.micro.service.payment"),
		micro.Version("latest"),
		micro.Address("0.0.0.0:8089"),
		// 添加注册中心
		micro.Registry(consul),
		// 添加链路追踪
		micro.WrapHandler(opentracing2.NewHandlerWrapper(opentracing.GlobalTracer())),
		// 加载限流
		micro.WrapHandler(ratelimit.NewHandlerWrapper(1000)),
		//加载监控
		micro.WrapHandler(prometheus.NewHandlerWrapper()),
	)

	// Initialise service
	service.Init()

	paymentDataService := service2.NewPaymentDataService(repository.NewPaymentRepository(db))

	// Register Handler
	payment.RegisterPaymentHandler(service.Server(), &handler.Payment{PaymentDataService: paymentDataService})

	// Register Struct as Subscriber
	//micro.RegisterSubscriber("go.micro.service.payment", service.Server(), new(subscriber.Payment))

	// Run service
	if err := service.Run(); err != nil {
		log.Fatal(err)
	}
}